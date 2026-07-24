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
	"testing"

	"github.com/google/uuid"
)

// T-021 / spec "Grupo de diretório sem mapeamento aprovado": pertencer a um grupo
// do diretório SEM mapeamento explícito e aprovado não confere alvo algum — papéis
// e concessões privilegiadas são SEMPRE do ArchGuard (design 009), jamais
// auto-derivados de grupo de diretório.
func TestDirectoryGroupWithoutApprovedMappingGrantsNothing(t *testing.T) {
	conn, err := NewDirectoryConnector(uuid.New(), DirectoryAD, "AD",
		"(objectClass=user)", "vault://k",
		[]AttributeMapping{{DirectoryAttr: "mail", ArchGuardAttr: "email"}},
		[]GroupMapping{
			{DirectoryGroup: "CN=DBA", TargetGroup: "dba", Approved: false}, // NÃO aprovado
		})
	if err != nil {
		t.Fatalf("connector: %v", err)
	}

	// Um usuário membro de CN=DBA no diretório...
	rec := DirectorySyncRecord{Email: "ana@cli.com", Groups: []string{"CN=DBA", "CN=Users"}, Active: true}

	// ...não recebe alvo por nenhum de seus grupos, pois nenhum está aprovado.
	for _, g := range rec.Groups {
		if _, ok := conn.ApprovedGroupTarget(g); ok {
			t.Fatalf("grupo %q sem mapeamento aprovado NÃO deveria conferir alvo", g)
		}
	}

	// Mesmo aprovando o mapeamento, ele mapeia para um GRUPO do ArchGuard — nunca
	// para um papel privilegiado (a derivação de papel privilegiado não existe aqui).
	_ = conn.ReviseMapping(conn.Mapping.Attributes, []GroupMapping{
		{DirectoryGroup: "CN=DBA", TargetGroup: "dba", Approved: true},
	})
	target, ok := conn.ApprovedGroupTarget("CN=DBA")
	if !ok || target != "dba" {
		t.Fatalf("grupo aprovado deveria mapear para o GRUPO alvo, veio %q ok=%v", target, ok)
	}
}
