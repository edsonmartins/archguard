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

package wormanchor

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

func sampleSeal() domain.Seal {
	head := make([]byte, domain.AuditHashSize)
	for i := range head {
		head[i] = byte(i)
	}
	return domain.Seal{
		OrganizationID: uuid.MustParse("018f9a00-0000-7000-8000-0000000000ff"),
		SeqStart:       1, SeqEnd: 100, HeadHash: head,
		SealedAt: 1_700_000_000_000_000, KeyID: "provisional-ed25519-1",
		Signature: []byte("assinatura-de-exemplo"),
	}
}

func TestMemoryAnchorAndFetch(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	seal := sampleSeal()

	ref, err := m.Anchor(ctx, seal)
	if err != nil {
		t.Fatalf("Anchor: %v", err)
	}
	// Fetch devolve o selo ancorado igual ao original.
	got, err := m.Fetch(ctx, ref)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got.OrganizationID != seal.OrganizationID || got.SeqEnd != seal.SeqEnd ||
		!bytes.Equal(got.HeadHash, seal.HeadHash) || got.KeyID != seal.KeyID ||
		!bytes.Equal(got.Signature, seal.Signature) {
		t.Fatalf("selo ancorado difere do original: %+v", got)
	}

	// Re-ancorar o mesmo selo é idempotente (mesma ref, mesmos bytes).
	ref2, err := m.Anchor(ctx, seal)
	if err != nil || ref2 != ref {
		t.Fatalf("re-ancoragem idempotente: ref=%s ref2=%s err=%v", ref, ref2, err)
	}
}

func TestMemoryRefusesOverwrite(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	seal := sampleSeal()
	ref, err := m.Anchor(ctx, seal)
	if err != nil {
		t.Fatalf("Anchor: %v", err)
	}

	// Força bytes diferentes sob a MESMA ref (write-once deve recusar).
	m.mu.Lock()
	m.objects[ref] = []byte("bytes-adulterados")
	m.mu.Unlock()
	if _, err := m.Anchor(ctx, seal); !errors.Is(err, ErrWORMOverwrite) {
		t.Fatalf("sobrescrita: err = %v, quero ErrWORMOverwrite", err)
	}
}

func TestSealExportRoundTrip(t *testing.T) {
	seal := sampleSeal()
	b, err := domain.MarshalSealExport(seal)
	if err != nil {
		t.Fatalf("MarshalSealExport: %v", err)
	}
	got, err := domain.UnmarshalSealExport(b)
	if err != nil {
		t.Fatalf("UnmarshalSealExport: %v", err)
	}
	if got.OrganizationID != seal.OrganizationID || got.SeqStart != seal.SeqStart ||
		got.SeqEnd != seal.SeqEnd || got.SealedAt != seal.SealedAt || got.KeyID != seal.KeyID {
		t.Fatalf("round-trip divergiu: %+v", got)
	}
	if !bytes.Equal(got.HeadHash, seal.HeadHash) || !bytes.Equal(got.Signature, seal.Signature) {
		t.Fatalf("bytes divergiram no round-trip")
	}
}
