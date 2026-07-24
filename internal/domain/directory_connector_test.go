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

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func newConnector(t *testing.T) DirectoryConnector {
	t.Helper()
	c, err := NewDirectoryConnector(uuid.New(), DirectoryAD, "AD Corporativo",
		"(&(objectClass=user)(memberOf=CN=ArchGuard,OU=Grupos,DC=cli,DC=com))",
		"vault://kv/data/org/ad-bind", nil, nil)
	if err != nil {
		t.Fatalf("NewDirectoryConnector: %v", err)
	}
	return c
}

func TestNewDirectoryConnectorValidates(t *testing.T) {
	org := uuid.New()
	// spec "Escopo não definido": conector sem filtro de escopo é rejeitado.
	if _, err := NewDirectoryConnector(org, DirectoryLDAP, "x", "", "vault://k", nil, nil); !errors.Is(err, ErrScopeFilterRequired) {
		t.Fatalf("sem filtro de escopo deveria ser rejeitado, veio %v", err)
	}
	// Credencial precisa ser custodiada (ref do cofre), nunca inline (INV-7).
	if _, err := NewDirectoryConnector(org, DirectoryLDAP, "x", "(uid=*)", "", nil, nil); !errors.Is(err, ErrCredentialRefRequired) {
		t.Fatalf("sem ref de credencial deveria ser rejeitado, veio %v", err)
	}
	if _, err := NewDirectoryConnector(org, "kerberos", "x", "(uid=*)", "vault://k", nil, nil); !errors.Is(err, ErrInvalidConnector) {
		t.Fatalf("tipo desconhecido deveria ser rejeitado")
	}
	if _, err := NewDirectoryConnector(uuid.Nil, DirectoryLDAP, "x", "(uid=*)", "vault://k", nil, nil); !errors.Is(err, ErrInvalidConnector) {
		t.Fatalf("sem organização deveria ser rejeitado")
	}
}

func TestNewDirectoryConnectorDefaults(t *testing.T) {
	c := newConnector(t)
	if c.Enabled {
		t.Fatalf("novo conector deveria nascer DESABILITADO (default seguro)")
	}
	if c.Mapping.Version != 1 {
		t.Fatalf("mapeamento deveria começar na versão 1, veio %d", c.Mapping.Version)
	}
}

func TestReviseMappingVersioning(t *testing.T) {
	c := newConnector(t)
	err := c.ReviseMapping(
		[]AttributeMapping{{DirectoryAttr: "mail", ArchGuardAttr: "email"}},
		[]GroupMapping{{DirectoryGroup: "CN=DBA", TargetGroup: "dba", Approved: true}},
	)
	if err != nil {
		t.Fatalf("ReviseMapping: %v", err)
	}
	if c.Mapping.Version != 2 {
		t.Fatalf("revisão deveria bump para versão 2, veio %d", c.Mapping.Version)
	}
	// Mapeamento incompleto é rejeitado (versão não avança).
	if err := c.ReviseMapping([]AttributeMapping{{DirectoryAttr: "mail"}}, nil); !errors.Is(err, ErrInvalidMapping) {
		t.Fatalf("atributo com lado vazio deveria ser rejeitado")
	}
	if c.Mapping.Version != 2 {
		t.Fatalf("uma revisão inválida não deveria avançar a versão")
	}
}

// Grupo sem mapeamento aprovado não confere alvo (spec "Grupo sem mapeamento
// aprovado"): papel privilegiado nunca é auto-derivado.
func TestApprovedGroupTarget(t *testing.T) {
	c := newConnector(t)
	_ = c.ReviseMapping(nil, []GroupMapping{
		{DirectoryGroup: "CN=DBA", TargetGroup: "dba", Approved: true},
		{DirectoryGroup: "CN=Temp", TargetGroup: "temp", Approved: false},
	})
	if target, ok := c.ApprovedGroupTarget("CN=DBA"); !ok || target != "dba" {
		t.Fatalf("grupo aprovado deveria resolver o alvo, veio %q ok=%v", target, ok)
	}
	if _, ok := c.ApprovedGroupTarget("CN=Temp"); ok {
		t.Fatalf("grupo NÃO aprovado não deveria conferir alvo")
	}
	if _, ok := c.ApprovedGroupTarget("CN=Inexistente"); ok {
		t.Fatalf("grupo sem mapeamento não deveria conferir alvo")
	}
}
