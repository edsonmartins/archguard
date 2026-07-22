// Copyright 2026 IntegrAllTech Ltda.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package domain

import "fmt"

// DivergenceKind classifies the first point where a chain fails verification
// (RFC-0003 §6): the report names the seq and the kind so an operator knows
// what happened.
type DivergenceKind string

const (
	// DivergenceNone: the chain (or the checked prefix) is intact.
	DivergenceNone DivergenceKind = ""
	// DivergenceRemoved: a seq is missing — the sequence is not consecutive, so
	// an event was removed (or the read is incomplete).
	DivergenceRemoved DivergenceKind = "removed"
	// DivergenceBrokenChain: an event's prev_hash does not link to the previous
	// event's hash (or the genesis) — a removal, reorder, or a tampered link.
	DivergenceBrokenChain DivergenceKind = "broken_chain"
	// DivergenceAltered: an event's content no longer hashes to its stored hash
	// — the event was modified.
	DivergenceAltered DivergenceKind = "altered"
	// DivergenceSealInvalid: a seal's signature does not verify, or its head
	// does not match the event at seq_end.
	DivergenceSealInvalid DivergenceKind = "seal_invalid"
)

// VerifyReport is the outcome of verifying a chain: OK, or the first divergence.
type VerifyReport struct {
	OK              bool
	EventsChecked   int
	SealsChecked    int
	FirstDivergence int64 // seq (or seq_end for a seal) of the first problem; 0 if OK
	Kind            DivergenceKind
	Detail          string
	// SealSignaturesChecked is true when the seal Ed25519 signatures were
	// verified (a seal verifier / vault was available). When false, only the
	// chain and the seal STRUCTURE (contiguity, head match) were checked —
	// enough to catch alteration, removal and reorder, but not a forged
	// signature (that needs the custodied public keys).
	SealSignaturesChecked bool
}

// ok builds an intact report.
func okReport(events, seals int) VerifyReport {
	return VerifyReport{OK: true, EventsChecked: events, SealsChecked: seals}
}

// diverged builds a failing report at seq.
func diverged(seq int64, kind DivergenceKind, format string, args ...any) VerifyReport {
	return VerifyReport{OK: false, FirstDivergence: seq, Kind: kind, Detail: fmt.Sprintf(format, args...)}
}

// VerifyChain recomputes the hash chain of an organization's events (assumed
// ordered by seq ascending) from the genesis hash and returns the FIRST
// divergence, or an intact report (RFC-0003 §6). It checks, in order:
//   - seq is consecutive from 1 (a gap ⇒ removed);
//   - prev_hash links to the running head (mismatch ⇒ broken chain);
//   - the stored hash recomputes from prev_hash + canonical content (mismatch ⇒
//     altered).
//
// The genesis is H(organization_id || genesis_nonce) (GenesisHash); the first
// event must chain from it.
func VerifyChain(genesis []byte, events []SealedEvent) VerifyReport {
	prev := genesis
	for i, e := range events {
		expected := int64(i + 1)
		if e.Seq != expected {
			return diverged(expected, DivergenceRemoved,
				"seq não consecutivo: esperado %d, veio %d", expected, e.Seq)
		}
		if !bytesEqual(e.PrevHash, prev) {
			return diverged(e.Seq, DivergenceBrokenChain,
				"prev_hash do evento %d não encadeia com o hash anterior", e.Seq)
		}
		recomputed, err := SealEvent(e.Event, e.PrevHash, e.Seq)
		if err != nil {
			return diverged(e.Seq, DivergenceAltered, "recomputação do evento %d falhou: %v", e.Seq, err)
		}
		if !bytesEqual(recomputed.Hash, e.Hash) {
			return diverged(e.Seq, DivergenceAltered,
				"conteúdo do evento %d não corresponde ao hash armazenado", e.Seq)
		}
		prev = e.Hash
	}
	return okReport(len(events), 0)
}
