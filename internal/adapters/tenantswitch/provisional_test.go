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

package tenantswitch

import (
	"context"
	"testing"

	"github.com/casdoor/casdoor/internal/deploy"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

func withProfile(t *testing.T, p deploy.Profile) {
	t.Helper()
	saved := deploy.Active()
	t.Cleanup(func() { deploy.SetActive(saved) })
	deploy.SetActive(p)
}

func TestProfilePolicyDevRequiresAAL1(t *testing.T) {
	withProfile(t, deploy.Dev)
	got, err := NewProfilePolicy().RequiredAAL(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("RequiredAAL: %v", err)
	}
	if got != domain.AAL1 {
		t.Fatalf("dev: AAL = %s, quero aal1", got)
	}
}

// Fora do dev, o selo provisório exige o nível MAIS FORTE até a política real
// (pacote 005): só pode sobre-exigir, nunca sub-exigir (fail-closed).
func TestProfilePolicyStrictOutsideDev(t *testing.T) {
	for _, p := range []deploy.Profile{deploy.Pilot, deploy.Production} {
		withProfile(t, p)
		got, err := NewProfilePolicy().RequiredAAL(context.Background(), uuid.New())
		if err != nil {
			t.Fatalf("%s: RequiredAAL: %v", p, err)
		}
		if got != domain.AAL3 {
			t.Fatalf("%s: AAL = %s, quero aal3 (o mais forte)", p, got)
		}
	}
}

func TestProfilePolicyRefusesNilOrganization(t *testing.T) {
	withProfile(t, deploy.Dev)
	if _, err := NewProfilePolicy().RequiredAAL(context.Background(), uuid.Nil); err == nil {
		t.Fatalf("organização nula deveria ser recusada")
	}
}

func TestMemorySwitchAuditorRecordsValidEvents(t *testing.T) {
	aud := NewMemorySwitchAuditor()
	ev := domain.TenantSwitchEvent{
		SessionID: uuid.New(), IdentityID: uuid.New(),
		FromMembershipID: uuid.New(), FromOrganizationID: uuid.New(),
		ToMembershipID: uuid.New(), ToOrganizationID: uuid.New(),
		ProvenAAL: domain.AAL1, TokenGeneration: 2,
	}
	if err := aud.RecordTenantSwitch(context.Background(), ev); err != nil {
		t.Fatalf("RecordTenantSwitch: %v", err)
	}
	if got := aud.Events(); len(got) != 1 || got[0].SessionID != ev.SessionID {
		t.Fatalf("evento não registrado: %+v", got)
	}

	// Evento malformado não é "auditado" como se fosse real.
	bad := ev
	bad.ToOrganizationID = uuid.Nil
	if err := aud.RecordTenantSwitch(context.Background(), bad); err == nil {
		t.Fatalf("evento inválido deveria ser recusado")
	}
	if len(aud.Events()) != 1 {
		t.Fatalf("evento inválido não pode entrar no registro")
	}
}
