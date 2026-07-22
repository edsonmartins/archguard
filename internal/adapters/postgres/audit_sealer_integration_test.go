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
	"fmt"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/adapters/auditseal"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func appendN(t *testing.T, w *AuditWriter, org uuid.UUID, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := w.Append(context.Background(), minimalInput(org)); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
}

// Selagem: sela o intervalo pendente, produz selo assinado verificável; re-selar
// sem novos eventos é no-op; um segundo selo é contíguo ao primeiro.
func TestAuditSealerSealOrganization(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	cleanupAudit(t, pool, org)

	signer, err := auditseal.NewProvisional()
	if err != nil {
		t.Fatalf("NewProvisional: %v", err)
	}
	w := NewAuditWriter(pool, fixedClock())
	sealer := NewAuditSealer(pool, signer, fixedClock())

	appendN(t, w, org, 3)
	seal, ok, err := sealer.SealOrganization(ctx, org)
	if err != nil || !ok {
		t.Fatalf("SealOrganization: ok=%v err=%v", ok, err)
	}
	if seal.SeqStart != 1 || seal.SeqEnd != 3 {
		t.Fatalf("intervalo do selo = [%d,%d], quero [1,3]", seal.SeqStart, seal.SeqEnd)
	}

	// O selo persistido é verificável: recomputa o conteúdo e confere a assinatura.
	var (
		seqStart, seqEnd int64
		headHash, sig    []byte
		keyID            string
		sealedAt         time.Time
	)
	if err := pool.QueryRow(ctx,
		`SELECT seq_start, seq_end, head_hash, sealed_at, key_id, signature
		 FROM audit_seal WHERE organization_id = $1 AND seq_end = 3`, org.String()).
		Scan(&seqStart, &seqEnd, &headHash, &sealedAt, &keyID, &sig); err != nil {
		t.Fatalf("leitura do selo: %v", err)
	}
	content, err := domain.SealContent(org, seqStart, seqEnd, headHash, sealedAt.UnixMicro())
	if err != nil {
		t.Fatalf("SealContent: %v", err)
	}
	valid, err := signer.Verify(ctx, content, sig, keyID)
	if err != nil || !valid {
		t.Fatalf("selo persistido não verifica: valid=%v err=%v", valid, err)
	}
	// Adulteração do conteúdo (head_hash) ⇒ assinatura não confere.
	bad := make([]byte, len(headHash))
	copy(bad, headHash)
	bad[0] ^= 0xFF
	badContent, _ := domain.SealContent(org, seqStart, seqEnd, bad, sealedAt.UnixMicro())
	if valid, _ := signer.Verify(ctx, badContent, sig, keyID); valid {
		t.Fatalf("conteúdo adulterado não deveria verificar")
	}

	// Re-selar sem eventos novos é no-op.
	if _, ok, err := sealer.SealOrganization(ctx, org); err != nil || ok {
		t.Fatalf("re-selagem sem novos eventos deveria ser no-op: ok=%v err=%v", ok, err)
	}

	// Novos eventos ⇒ segundo selo contíguo [4,5].
	appendN(t, w, org, 2)
	seal2, ok, err := sealer.SealOrganization(ctx, org)
	if err != nil || !ok {
		t.Fatalf("segunda selagem: ok=%v err=%v", ok, err)
	}
	if seal2.SeqStart != 4 || seal2.SeqEnd != 5 {
		t.Fatalf("segundo selo = [%d,%d], quero [4,5] (contíguo)", seal2.SeqStart, seal2.SeqEnd)
	}
}

// SealDue dispara por VOLUME e por INTERVALO.
func TestAuditSealerSealDue(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	signer, err := auditseal.NewProvisional()
	if err != nil {
		t.Fatalf("NewProvisional: %v", err)
	}

	// Relógio dos eventos: um tempo-base T0.
	t0 := time.Unix(1_700_000_000, 0).UTC()
	writer := NewAuditWriter(pool, func() time.Time { return t0 })

	// Org com VOLUME suficiente, avaliada "agora" (T0) — vence por volume.
	orgVol := uuid.New()
	cleanupAudit(t, pool, orgVol)
	appendN(t, writer, orgVol, 5)

	// Org com POUCOS eventos, mas antiga — vence por intervalo (avaliada em T0+2h).
	orgOld := uuid.New()
	cleanupAudit(t, pool, orgOld)
	appendN(t, writer, orgOld, 2)

	// Org com poucos eventos e recente — NÃO vence (nem volume nem intervalo).
	orgFresh := uuid.New()
	cleanupAudit(t, pool, orgFresh)

	// SealDue avaliada em T0+2h, volume=5, intervalo=1h.
	sealerAt := NewAuditSealer(pool, signer, func() time.Time { return t0.Add(2 * time.Hour) })
	// orgFresh recebe seus eventos JÁ em T0+2h (recente) para não vencer por intervalo.
	freshWriter := NewAuditWriter(pool, func() time.Time { return t0.Add(2 * time.Hour) })
	appendN(t, freshWriter, orgFresh, 2)

	n, err := sealerAt.SealDue(ctx, SealConfig{Volume: 5, Interval: time.Hour})
	if err != nil {
		t.Fatalf("SealDue: %v", err)
	}
	// Vencem orgVol (volume=5) e orgOld (intervalo); orgFresh não.
	if n != 2 {
		t.Fatalf("SealDue selou %d, quero 2 (volume + intervalo)", n)
	}
	if sealedRange(t, pool, orgVol) == "" {
		t.Fatalf("orgVol deveria ter sido selada (volume)")
	}
	if sealedRange(t, pool, orgOld) == "" {
		t.Fatalf("orgOld deveria ter sido selada (intervalo)")
	}
	if sealedRange(t, pool, orgFresh) != "" {
		t.Fatalf("orgFresh NÃO deveria ter sido selada")
	}
}

// sealedRange returns "start-end" of the org's latest seal, or "" if none.
func sealedRange(t *testing.T, pool *pgxpool.Pool, org uuid.UUID) string {
	t.Helper()
	var start, end *int64
	if err := pool.QueryRow(context.Background(),
		"SELECT MIN(seq_start), MAX(seq_end) FROM audit_seal WHERE organization_id = $1", org.String()).
		Scan(&start, &end); err != nil {
		t.Fatalf("sealedRange: %v", err)
	}
	if start == nil || end == nil {
		return ""
	}
	return fmt.Sprintf("%d-%d", *start, *end)
}
