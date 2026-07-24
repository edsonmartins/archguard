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

func TestMappingRejectsDuplicates(t *testing.T) {
	c := newConnector(t)
	// Mesmo atributo de diretório mapeado duas vezes.
	if err := c.ReviseMapping([]AttributeMapping{
		{DirectoryAttr: "mail", ArchGuardAttr: "email"},
		{DirectoryAttr: "mail", ArchGuardAttr: "outro"},
	}, nil); !errors.Is(err, ErrInvalidMapping) {
		t.Fatalf("atributo de diretório duplicado deveria ser rejeitado, veio %v", err)
	}
	// Mesmo atributo-alvo recebendo dois mapeamentos.
	if err := c.ReviseMapping([]AttributeMapping{
		{DirectoryAttr: "mail", ArchGuardAttr: "email"},
		{DirectoryAttr: "userPrincipalName", ArchGuardAttr: "email"},
	}, nil); !errors.Is(err, ErrInvalidMapping) {
		t.Fatalf("atributo-alvo duplicado deveria ser rejeitado, veio %v", err)
	}
	// Grupo de diretório duplicado.
	if err := c.ReviseMapping(nil, []GroupMapping{
		{DirectoryGroup: "CN=DBA", TargetGroup: "dba", Approved: true},
		{DirectoryGroup: "CN=DBA", TargetGroup: "outro", Approved: false},
	}); !errors.Is(err, ErrInvalidMapping) {
		t.Fatalf("grupo de diretório duplicado deveria ser rejeitado, veio %v", err)
	}
}

// Um conector só pode SINCRONIZAR / ser habilitado se mapear e-mail (dedup).
func TestValidateForSyncRequiresEmail(t *testing.T) {
	c := newConnector(t) // criado com mapeamento vazio
	if err := c.ValidateForSync(); !errors.Is(err, ErrEmailMappingRequired) {
		t.Fatalf("sem mapeamento de e-mail não deveria poder sincronizar, veio %v", err)
	}
	if err := c.Enable(); !errors.Is(err, ErrEmailMappingRequired) {
		t.Fatalf("Enable sem e-mail deveria falhar, veio %v", err)
	}
	if c.Enabled {
		t.Fatalf("conector não deveria ter sido habilitado")
	}

	// Após mapear e-mail, habilita.
	if err := c.ReviseMapping([]AttributeMapping{{DirectoryAttr: "mail", ArchGuardAttr: "email"}}, nil); err != nil {
		t.Fatalf("ReviseMapping: %v", err)
	}
	if err := c.Enable(); err != nil {
		t.Fatalf("com e-mail mapeado deveria habilitar: %v", err)
	}
	if !c.Enabled {
		t.Fatalf("conector deveria estar habilitado")
	}
}

// Construção com mapeamento inicial vazio continua permitida (só não sincroniza).
func TestConnectorConstructsWithEmptyMapping(t *testing.T) {
	if _, err := NewDirectoryConnector(uuid.New(), DirectoryLDAP, "x", "(uid=*)", "vault://k", nil, nil); err != nil {
		t.Fatalf("construção com mapeamento vazio deveria ser permitida: %v", err)
	}
}
