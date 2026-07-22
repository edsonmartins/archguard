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
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/auditseal"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// bypassExec runs a statement with the append-only triggers disabled (superuser
// only) — the "tampering directly in the database" the verifier must catch.
func bypassExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SET session_replication_role = replica"); err != nil {
		t.Fatalf("set replica: %v", err)
	}
	if _, err := conn.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("bypass exec: %v", err)
	}
	_, _ = conn.Exec(ctx, "SET session_replication_role = origin")
}

func setupVerifiable(t *testing.T, label string) (*pgxpool.Pool, uuid.UUID, *AuditVerifier, *AuditSealer, *AuditWriter) {
	t.Helper()
	pool := setupTenantPool(t)
	org := uuid.New()
	cleanupAudit(t, pool, org)
	signer, err := auditseal.NewProvisional()
	if err != nil {
		t.Fatalf("NewProvisional: %v", err)
	}
	w := NewAuditWriter(pool, fixedClock())
	sealer := NewAuditSealer(pool, signer, fixedClock())
	verifier := NewAuditVerifier(pool, signer)
	return pool, org, verifier, sealer, w
}

// Cadeia íntegra e selada verifica sem divergência.
func TestAuditVerifierIntact(t *testing.T) {
	_, org, verifier, sealer, w := setupVerifiable(t, "intact")
	ctx := context.Background()
	appendN(t, w, org, 4)
	if _, ok, err := sealer.SealOrganization(ctx, org); err != nil || !ok {
		t.Fatalf("selo: ok=%v err=%v", ok, err)
	}
	rep, err := verifier.VerifyOrganization(ctx, org)
	if err != nil {
		t.Fatalf("VerifyOrganization: %v", err)
	}
	if !rep.OK || rep.EventsChecked != 4 || rep.SealsChecked != 1 {
		t.Fatalf("cadeia íntegra deveria verificar: %+v", rep)
	}
}

// Cenário "Evento alterado diretamente no banco": o verificador aponta o seq.
func TestAuditVerifierDetectsAlteredEvent(t *testing.T) {
	pool, org, verifier, _, w := setupVerifiable(t, "altered")
	ctx := context.Background()
	appendN(t, w, org, 4)

	// Adultera o conteúdo do evento seq 2 (sem recomputar o hash).
	bypassExec(t, pool, "UPDATE audit_event SET reason = 'adulterado' WHERE organization_id = $1 AND seq = 2", org.String())

	rep, err := verifier.VerifyOrganization(ctx, org)
	if err != nil {
		t.Fatalf("VerifyOrganization: %v", err)
	}
	if rep.OK || rep.Kind != domain.DivergenceAltered || rep.FirstDivergence != 2 {
		t.Fatalf("alteração deveria ser detectada no seq 2: %+v", rep)
	}
}

// Cenário "Evento removido diretamente no banco": lacuna de sequência.
func TestAuditVerifierDetectsRemovedEvent(t *testing.T) {
	pool, org, verifier, _, w := setupVerifiable(t, "removed")
	ctx := context.Background()
	appendN(t, w, org, 4)

	bypassExec(t, pool, "DELETE FROM audit_event WHERE organization_id = $1 AND seq = 3", org.String())

	rep, err := verifier.VerifyOrganization(ctx, org)
	if err != nil {
		t.Fatalf("VerifyOrganization: %v", err)
	}
	if rep.OK || rep.Kind != domain.DivergenceRemoved || rep.FirstDivergence != 3 {
		t.Fatalf("remoção deveria ser detectada no seq 3: %+v", rep)
	}
}

// Cenário "Selo inválido": adulterar um evento já SELADO é pego pela assinatura
// do selo (o head_hash assinado não bate mais).
func TestAuditVerifierDetectsSealMismatch(t *testing.T) {
	pool, org, verifier, sealer, w := setupVerifiable(t, "sealbad")
	ctx := context.Background()
	appendN(t, w, org, 3)
	if _, ok, err := sealer.SealOrganization(ctx, org); err != nil || !ok {
		t.Fatalf("selo: ok=%v err=%v", ok, err)
	}

	// Adultera o head_hash do selo diretamente (assinatura não confere mais).
	bypassExec(t, pool,
		"UPDATE audit_seal SET head_hash = decode('00', 'hex') || head_hash WHERE organization_id = $1", org.String())

	rep, err := verifier.VerifyOrganization(ctx, org)
	if err != nil {
		t.Fatalf("VerifyOrganization: %v", err)
	}
	if rep.OK || rep.Kind != domain.DivergenceSealInvalid {
		t.Fatalf("selo adulterado deveria ser detectado: %+v", rep)
	}
}

// Reordenação: trocar o prev_hash de um elo quebra a cadeia.
func TestAuditVerifierDetectsBrokenChain(t *testing.T) {
	pool, org, verifier, _, w := setupVerifiable(t, "broken")
	ctx := context.Background()
	appendN(t, w, org, 4)

	bypassExec(t, pool,
		"UPDATE audit_event SET prev_hash = decode(repeat('00',32),'hex') WHERE organization_id = $1 AND seq = 3", org.String())

	rep, err := verifier.VerifyOrganization(ctx, org)
	if err != nil {
		t.Fatalf("VerifyOrganization: %v", err)
	}
	if rep.OK || rep.Kind != domain.DivergenceBrokenChain || rep.FirstDivergence != 3 {
		t.Fatalf("quebra de cadeia deveria ser detectada no seq 3: %+v", rep)
	}
}
