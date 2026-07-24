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

type fakeSCIMGroupProvisioner struct {
	gotOrg uuid.UUID
	gotRec domain.GroupSyncRecord
	id     string
	err    error
}

func (p *fakeSCIMGroupProvisioner) ProvisionGroup(_ context.Context, orgID uuid.UUID, rec domain.GroupSyncRecord) (string, error) {
	p.gotOrg, p.gotRec = orgID, rec
	return p.id, p.err
}

const scimGroupHTTPBody = `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"DBA","members":[{"value":"u1"},{"value":"u2"}]}`

func TestSCIMGroupCreate(t *testing.T) {
	org := uuid.New()
	prov := &fakeSCIMGroupProvisioner{id: "g-1"}
	h := NewSCIMGroupHandler(prov, fixedOrg(org, true))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", strings.NewReader(scimGroupHTTPBody))
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("esperava 201, veio %d (%s)", rr.Code, rr.Body.String())
	}
	if prov.gotOrg != org || prov.gotRec.DisplayName != "DBA" || len(prov.gotRec.MemberIDs) != 2 {
		t.Fatalf("provisioner recebeu registro inesperado: %+v", prov.gotRec)
	}
	var resp domain.SCIMGroup
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("resposta não é SCIM Group: %v", err)
	}
	if resp.ID != "g-1" || resp.Meta == nil || resp.Meta.ResourceType != "Group" {
		t.Fatalf("resposta sem id/meta: %+v", resp)
	}
}

func TestSCIMGroupBadRequest(t *testing.T) {
	h := NewSCIMGroupHandler(&fakeSCIMGroupProvisioner{}, fixedOrg(uuid.New(), true))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", strings.NewReader(`{"schemas":["x"]}`))
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), scimErrorSchema) {
		t.Fatalf("esperava 400 SCIM, veio %d %s", rr.Code, rr.Body.String())
	}
}
