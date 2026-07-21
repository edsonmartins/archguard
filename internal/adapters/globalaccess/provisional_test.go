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

package globalaccess

import (
	"context"
	"errors"
	"testing"

	"github.com/casdoor/casdoor/internal/deploy"
	"github.com/casdoor/casdoor/internal/domain"
)

func TestProfileAuthorizerAllowsOnlyInDev(t *testing.T) {
	saved := deploy.Active()
	t.Cleanup(func() { deploy.SetActive(saved) })

	a := NewProfileAuthorizer()
	access := domain.GlobalAccess{Principal: "op", Reason: "relatório global"}

	deploy.SetActive(deploy.Dev)
	if err := a.Authorize(context.Background(), access); err != nil {
		t.Errorf("dev deveria permitir, veio: %v", err)
	}

	// Fora de dev, fail-closed (INV-6): nega até o PDP real (pacote 007).
	for _, p := range []deploy.Profile{deploy.Pilot, deploy.Production} {
		deploy.SetActive(p)
		if err := a.Authorize(context.Background(), access); !errors.Is(err, domain.ErrGlobalAccessDenied) {
			t.Errorf("perfil %v deveria negar, veio: %v", p, err)
		}
	}
}

func TestProfileAuthorizerRejectsIllFormedAccess(t *testing.T) {
	saved := deploy.Active()
	t.Cleanup(func() { deploy.SetActive(saved) })
	deploy.SetActive(deploy.Dev)

	a := NewProfileAuthorizer()
	if err := a.Authorize(context.Background(), domain.GlobalAccess{Reason: "sem principal"}); !errors.Is(err, domain.ErrGlobalAccessDenied) {
		t.Error("acesso sem principal deveria ser negado mesmo em dev")
	}
}

func TestMemoryAuditorRecords(t *testing.T) {
	m := NewMemoryAuditor()
	access := domain.GlobalAccess{Principal: "op", Reason: "auditoria trimestral"}
	if err := m.Record(context.Background(), access); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if recs := m.Records(); len(recs) != 1 || recs[0] != access {
		t.Errorf("registros = %+v", m.Records())
	}
	// Acesso malformado não é registrado como evento real.
	if err := m.Record(context.Background(), domain.GlobalAccess{}); err == nil {
		t.Error("acesso malformado deveria ser rejeitado")
	}
	if len(m.Records()) != 1 {
		t.Error("acesso malformado não deveria ter sido registrado")
	}
}
