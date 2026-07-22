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
	"strings"
	"testing"

	"github.com/casdoor/casdoor/internal/adapters/alerting"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// A verificação diária percorre todas as orgs; numa trilha íntegra não alerta,
// e numa adulterada emite alerta CRÍTICO e retorna erro (sinal de incidente).
func TestAuditVerificationJobRunOnce(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	orgGood := uuid.New()
	orgBad := uuid.New()
	cleanupAudit(t, pool, orgGood, orgBad)

	w := NewAuditWriter(pool, fixedClock())
	appendN(t, w, orgGood, 3)
	appendN(t, w, orgBad, 3)

	alerter := alerting.NewMemoryAlerter()
	job := NewAuditVerificationJob(pool, NewAuditVerifier(pool, nil), alerter)

	// Antes da adulteração: nenhuma divergência, nenhum alerta.
	sum, err := job.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce íntegra: %v", err)
	}
	if sum.Divergences != 0 || len(alerter.Alerts()) != 0 {
		t.Fatalf("trilha íntegra não deveria alertar: %+v / %d alertas", sum, len(alerter.Alerts()))
	}
	if sum.OrgsChecked < 2 {
		t.Fatalf("deveria checar ao menos as 2 orgs, checou %d", sum.OrgsChecked)
	}

	// Adultera a org "bad".
	bypassExec(t, pool, "UPDATE audit_event SET reason = 'x' WHERE organization_id = $1 AND seq = 2", orgBad.String())

	sum, err = job.RunOnce(ctx)
	if err == nil {
		t.Fatalf("RunOnce com adulteração deveria retornar erro (incidente)")
	}
	if sum.Divergences != 1 {
		t.Fatalf("divergências = %d, quero 1", sum.Divergences)
	}
	// Alerta crítico emitido, mencionando a org adulterada.
	var critical *domain.Alert
	for i := range alerter.Alerts() {
		a := alerter.Alerts()[i]
		if a.Severity == domain.SeverityCritical && strings.Contains(a.Detail, orgBad.String()) {
			critical = &a
		}
	}
	if critical == nil {
		t.Fatalf("deveria ter alerta CRÍTICO para a org adulterada: %+v", alerter.Alerts())
	}
	if !strings.Contains(critical.Subject, "ADULTERAÇÃO") {
		t.Fatalf("assunto do alerta inesperado: %q", critical.Subject)
	}
}

// Falha ao verificar (não só divergência) também alerta crítico — fail-closed.
func TestAuditVerificationJobAlertsOnVerifyError(t *testing.T) {
	pool := setupTenantPool(t)
	ctx := context.Background()
	org := uuid.New()
	cleanupAudit(t, pool, org)
	w := NewAuditWriter(pool, fixedClock())
	appendN(t, w, org, 2)

	// Corrompe o genesis_nonce (tamanho inválido) → GenesisHash falha ao verificar.
	bypassExec(t, pool, "UPDATE audit_chain_head SET genesis_nonce = decode('00','hex') WHERE organization_id = $1", org.String())

	alerter := alerting.NewMemoryAlerter()
	job := NewAuditVerificationJob(pool, NewAuditVerifier(pool, nil), alerter)
	sum, err := job.RunOnce(ctx)
	if err == nil || sum.VerifyErrors != 1 {
		t.Fatalf("falha de verificação deveria contar e retornar erro: sum=%+v err=%v", sum, err)
	}
	found := false
	for _, a := range alerter.Alerts() {
		if a.Severity == domain.SeverityCritical {
			found = true
		}
	}
	if !found {
		t.Fatalf("falha de verificação deveria alertar crítico")
	}
}
