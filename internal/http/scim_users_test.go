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

package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type fakeSCIMProvisioner struct {
	gotOrg uuid.UUID
	gotRec domain.DirectorySyncRecord
	id     string
	err    error
}

func (p *fakeSCIMProvisioner) ProvisionUser(_ context.Context, orgID uuid.UUID, rec domain.DirectorySyncRecord) (string, error) {
	p.gotOrg, p.gotRec = orgID, rec
	return p.id, p.err
}

func fixedOrg(org uuid.UUID, ok bool) OrgResolver {
	return func(*http.Request) (uuid.UUID, bool) { return org, ok }
}

const scimBody = `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"ana","emails":[{"value":"ana@cli.com","primary":true}],"active":true}`

// Criação via SCIM: cria e responde 201 conforme SCIM 2.0 (id + meta), delegando
// o registro neutro ao provisioner (spec "Criação via SCIM").
func TestSCIMUserCreate(t *testing.T) {
	org := uuid.New()
	prov := &fakeSCIMProvisioner{id: "id-123"}
	h := NewSCIMUserHandler(prov, fixedOrg(org, true))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users", strings.NewReader(scimBody))
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("esperava 201, veio %d (%s)", rr.Code, rr.Body.String())
	}
	if prov.gotOrg != org || prov.gotRec.Email != "ana@cli.com" {
		t.Fatalf("provisioner recebeu org/registro inesperados: %v %+v", prov.gotOrg, prov.gotRec)
	}
	var resp domain.SCIMUser
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("resposta não é SCIM User: %v", err)
	}
	if resp.ID != "id-123" || resp.Meta == nil || resp.Meta.ResourceType != "User" {
		t.Fatalf("resposta SCIM sem id/meta: %+v", resp)
	}
	if rr.Header().Get("Content-Type") != "application/scim+json" {
		t.Fatalf("content-type deveria ser scim+json")
	}
}

// Corpo inválido → 400 em formato de erro SCIM.
func TestSCIMUserBadRequest(t *testing.T) {
	h := NewSCIMUserHandler(&fakeSCIMProvisioner{}, fixedOrg(uuid.New(), true))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users", strings.NewReader(`{"schemas":["x"],"userName":"a"}`))
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("esperava 400, veio %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), scimErrorSchema) {
		t.Fatalf("erro deveria estar em formato SCIM: %s", rr.Body.String())
	}
}

// Sem tenant resolvido → 401 (não cria nada).
func TestSCIMUserNoTenant(t *testing.T) {
	prov := &fakeSCIMProvisioner{id: "x"}
	h := NewSCIMUserHandler(prov, fixedOrg(uuid.Nil, false))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users", strings.NewReader(scimBody))
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("esperava 401, veio %d", rr.Code)
	}
	if prov.gotRec.Email != "" {
		t.Fatalf("não deveria ter chamado o provisioner sem tenant")
	}
}
