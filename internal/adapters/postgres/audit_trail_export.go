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

package postgres

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TrailExporter produces a portable, SELF-VERIFIABLE export of one
// organization's audit trail (RFC-0003 §9, T-016): an NDJSON stream carrying the
// genesis, the public keys, every event (with its chain links) and every seal,
// plus the verification procedure. An external auditor / SIEM can recompute the
// chain and verify the seal signatures OFFLINE, with nothing from ArchGuard but
// this stream. Exporting the trail is an L3 audited operation (the audit-of-
// export event is instrumented in T-017).
type TrailExporter struct {
	db Beginner
}

// NewTrailExporter builds the exporter.
func NewTrailExporter(db Beginner) *TrailExporter { return &TrailExporter{db: db} }

// PublicKeyResolver returns the public key (raw bytes) for a seal's key_id, so
// the export can carry it for offline signature verification. In production it
// reads the vault's public keys; a dev caller wraps the provisional signer.
type PublicKeyResolver func(keyID string) ([]byte, bool)

// TrailExportAssuranceLevel declares this operation's assurance level (INV-8 /
// ADR-0010): exporting the trail is privileged — L3.
const TrailExportAssuranceLevel = domain.L3

// Export writes the organization's trail as NDJSON to w. Each line is a typed
// record: one "meta" (with the genesis), one "public_key" per key_id used by the
// seals, one "event" per event (seq order) carrying the full canonical event
// plus its prev_hash/hash, one "seal" per seal, and one "procedure". The bytes
// are hex so the stream is plain text. clock stamps the export time.
func (x *TrailExporter) Export(ctx context.Context, orgID uuid.UUID, resolve PublicKeyResolver, clock Clock, w io.Writer) error {
	if clock == nil {
		clock = time.Now
	}
	genesis, hasHead, err := readGenesisHash(ctx, x.db, orgID)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)

	if err := enc.Encode(exportMeta{
		Type:           "meta",
		OrganizationID: orgID.String(),
		ExportedAtUS:   clock().UTC().UnixMicro(),
		SchemaVersion:  domain.AuditSchemaVersion,
		GenesisHashHex: hexOrEmpty(genesis),
		HasChain:       hasHead,
	}); err != nil {
		return fmt.Errorf("audit: escrita do meta do export falhou: %w", err)
	}

	seals, keyIDs, err := x.readSeals(ctx, orgID)
	if err != nil {
		return err
	}
	// Public keys used by the seals, so signatures verify offline.
	for _, keyID := range keyIDs {
		pub, ok := resolve(keyID)
		if !ok {
			return fmt.Errorf("audit: chave pública do key_id %q não disponível para exportação", keyID)
		}
		if err := enc.Encode(exportPublicKey{Type: "public_key", KeyID: keyID, PublicKeyHex: hex.EncodeToString(pub)}); err != nil {
			return fmt.Errorf("audit: escrita de chave pública falhou: %w", err)
		}
	}

	events, err := readSealedEvents(ctx, x.db, orgID)
	if err != nil {
		return err
	}
	for _, e := range events {
		if err := enc.Encode(exportEvent{
			Type:        "event",
			Seq:         e.Seq,
			PrevHashHex: hex.EncodeToString(e.PrevHash),
			HashHex:     hex.EncodeToString(e.Hash),
			Event:       e.Event,
		}); err != nil {
			return fmt.Errorf("audit: escrita de evento falhou: %w", err)
		}
	}

	for _, s := range seals {
		if err := enc.Encode(s); err != nil {
			return fmt.Errorf("audit: escrita de selo falhou: %w", err)
		}
	}

	if err := enc.Encode(exportProcedure{Type: "procedure", Text: verificationProcedure}); err != nil {
		return fmt.Errorf("audit: escrita do procedimento falhou: %w", err)
	}
	return nil
}

// readSeals returns the org's seals as export records (seq_start order) and the
// distinct key_ids they reference.
func (x *TrailExporter) readSeals(ctx context.Context, orgID uuid.UUID) ([]exportSeal, []string, error) {
	var out []exportSeal
	seen := map[string]bool{}
	var keyIDs []string
	err := WithTx(ctx, x.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT seq_start, seq_end, head_hash, sealed_at, key_id, signature
			FROM audit_seal WHERE organization_id = $1 ORDER BY seq_start`, orgID.String())
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var (
				s        exportSeal
				headHash []byte
				sig      []byte
				sealedAt time.Time
			)
			if err := rows.Scan(&s.SeqStart, &s.SeqEnd, &headHash, &sealedAt, &s.KeyID, &sig); err != nil {
				return err
			}
			s.Type = "seal"
			s.HeadHashHex = hex.EncodeToString(headHash)
			s.SignatureHex = hex.EncodeToString(sig)
			s.SealedAtUS = sealedAt.UTC().UnixMicro()
			out = append(out, s)
			if !seen[s.KeyID] {
				seen[s.KeyID] = true
				keyIDs = append(keyIDs, s.KeyID)
			}
		}
		return rows.Err()
	})
	if err != nil {
		return nil, nil, fmt.Errorf("audit: leitura de selos para exportação falhou: %w", err)
	}
	return out, keyIDs, nil
}

func hexOrEmpty(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return hex.EncodeToString(b)
}

// Export record shapes (NDJSON, one per line, distinguished by "type").
type exportMeta struct {
	Type           string `json:"type"`
	OrganizationID string `json:"organization_id"`
	ExportedAtUS   int64  `json:"exported_at_us"`
	SchemaVersion  int    `json:"schema_version"`
	GenesisHashHex string `json:"genesis_hash_hex"`
	HasChain       bool   `json:"has_chain"`
}

type exportPublicKey struct {
	Type         string `json:"type"`
	KeyID        string `json:"key_id"`
	PublicKeyHex string `json:"public_key_hex"`
}

type exportEvent struct {
	Type        string            `json:"type"`
	Seq         int64             `json:"seq"`
	PrevHashHex string            `json:"prev_hash_hex"`
	HashHex     string            `json:"hash_hex"`
	Event       domain.AuditEvent `json:"event"`
}

type exportSeal struct {
	Type         string `json:"type"`
	SeqStart     int64  `json:"seq_start"`
	SeqEnd       int64  `json:"seq_end"`
	HeadHashHex  string `json:"head_hash_hex"`
	SealedAtUS   int64  `json:"sealed_at_us"`
	KeyID        string `json:"key_id"`
	SignatureHex string `json:"signature_hex"`
}

type exportProcedure struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// verificationProcedure is the offline verification recipe carried in the
// export so an auditor can reproduce it without ArchGuard.
const verificationProcedure = `Procedimento de verificação (RFC-0003 §6):
1. Leia o registro "meta": obtenha organization_id e genesis_hash_hex.
2. Para cada "event" em ordem de seq (1..N):
   a. canonical = JSON canônico do campo "event" (chaves ordenadas, UTF-8 NFC,
      occurred_at em microssegundos inteiros; ver schema_version);
   b. esperado_hash = SHA-256( prev_hash_do_evento || canonical );
   c. confirme esperado_hash == hash_hex; confirme prev_hash_hex == hash do
      evento anterior (ou genesis_hash_hex no primeiro); confirme seq consecutivo.
3. Para cada "seal": seal_content = JSON canônico de {organization_id, seq_start,
   seq_end, head_hash(hex), sealed_at_us}; verifique a assinatura Ed25519 com a
   "public_key" cujo key_id corresponde; confirme head_hash == hash do evento em
   seq_end e a contiguidade dos intervalos.
Qualquer divergência indica adulteração (alteração, remoção, reordenação ou
selo inválido).`
