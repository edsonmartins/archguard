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

package ldapsync

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"
)

type fakeSearcher struct {
	lastReq *ldap.SearchRequest
	result  *ldap.SearchResult
	err     error
}

func (f *fakeSearcher) Search(req *ldap.SearchRequest) (*ldap.SearchResult, error) {
	f.lastReq = req
	return f.result, f.err
}

func adConnector(t *testing.T) domain.DirectoryConnector {
	t.Helper()
	c, err := domain.NewDirectoryConnector(uuid.New(), domain.DirectoryAD, "AD Corp",
		"(objectClass=user)", "vault://kv/bind",
		[]domain.AttributeMapping{
			{DirectoryAttr: "mail", ArchGuardAttr: "email"},
			{DirectoryAttr: "displayName", ArchGuardAttr: "name"},
		}, nil)
	if err != nil {
		t.Fatalf("connector: %v", err)
	}
	return c
}

func TestSyncMapsEntriesAndDetectsDisabled(t *testing.T) {
	conn := adConnector(t)
	fs := &fakeSearcher{result: &ldap.SearchResult{Entries: []*ldap.Entry{
		ldap.NewEntry("CN=Ana,OU=Users,DC=cli,DC=com", map[string][]string{
			"mail":               {"ana@cli.com"},
			"displayName":        {"Ana Souza"},
			"memberOf":           {"CN=DBA,OU=Grp,DC=cli,DC=com", "CN=Users,OU=Grp,DC=cli,DC=com"},
			"objectGUID":         {"guid-ana"},
			"modifyTimestamp":    {"20260723120000Z"},
			"userAccountControl": {"512"}, // conta normal (habilitada)
		}),
		ldap.NewEntry("CN=Bob,OU=Users,DC=cli,DC=com", map[string][]string{
			"mail":               {"bob@cli.com"},
			"objectGUID":         {"guid-bob"},
			"modifyTimestamp":    {"20260723130000Z"},
			"userAccountControl": {"514"}, // 512 + 0x2 ACCOUNTDISABLE = desabilitada
		}),
	}}}

	res, err := NewSyncer().Sync(context.Background(), fs, "OU=Users,DC=cli,DC=com", conn, time.Time{})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Sync inicial (since zero): o filtro é só o de escopo, sem predicado incremental.
	if fs.lastReq.Filter != conn.ScopeFilter {
		t.Fatalf("sync inicial deveria usar só o filtro de escopo, veio %q", fs.lastReq.Filter)
	}
	if fs.lastReq.BaseDN != "OU=Users,DC=cli,DC=com" {
		t.Fatalf("base de busca inesperada: %q", fs.lastReq.BaseDN)
	}

	if len(res.Records) != 2 {
		t.Fatalf("esperava 2 registros, veio %d", len(res.Records))
	}
	ana := res.Records[0]
	if ana.Email != "ana@cli.com" || ana.Attributes["name"] != "Ana Souza" || ana.ExternalID != "guid-ana" {
		t.Fatalf("mapeamento da Ana inesperado: %+v", ana)
	}
	if !ana.Active {
		t.Fatalf("conta habilitada (uac 512) deveria ser ativa")
	}
	if len(ana.Groups) != 2 || !strings.Contains(ana.Groups[0], "CN=DBA") {
		t.Fatalf("grupos da Ana inesperados: %+v", ana.Groups)
	}
	bob := res.Records[1]
	if bob.Active {
		t.Fatalf("conta com ACCOUNTDISABLE (uac 514) deveria ser INATIVA (dispara suspensão)")
	}

	// High-water é o maior modifyTimestamp visto.
	want := time.Date(2026, 7, 23, 13, 0, 0, 0, time.UTC)
	if !res.HighWater.Equal(want) {
		t.Fatalf("high-water deveria ser %v, veio %v", want, res.HighWater)
	}
}

// Sync incremental: com `since`, o filtro combina escopo AND modifyTimestamp.
func TestSyncIncrementalFilter(t *testing.T) {
	conn := adConnector(t)
	fs := &fakeSearcher{result: &ldap.SearchResult{}}
	since := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)

	if _, err := NewSyncer().Sync(context.Background(), fs, "DC=cli,DC=com", conn, since); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	f := fs.lastReq.Filter
	if !strings.HasPrefix(f, "(&(objectClass=user)(modifyTimestamp>=20260723100000Z") {
		t.Fatalf("filtro incremental inesperado: %q", f)
	}
}

// Base de busca vazia é recusada (nunca buscar sem raiz de subárvore).
func TestSyncRequiresSearchBase(t *testing.T) {
	conn := adConnector(t)
	fs := &fakeSearcher{result: &ldap.SearchResult{}}
	if _, err := NewSyncer().Sync(context.Background(), fs, "", conn, time.Time{}); err == nil {
		t.Fatalf("base de busca vazia deveria ser recusada")
	}
}
