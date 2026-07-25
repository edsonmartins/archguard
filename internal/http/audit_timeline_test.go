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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type fakeAuditReader struct {
	events   []domain.SealedEvent
	err      error
	gotOrg   uuid.UUID
	gotLimit int
}

func (f *fakeAuditReader) ListRecent(_ context.Context, org uuid.UUID, limit int) ([]domain.SealedEvent, error) {
	f.gotOrg, f.gotLimit = org, limit
	return f.events, f.err
}

func auditTimelineReq(orgID uuid.UUID, query string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/audit/timeline"+query, nil)
	return req.WithContext(withSession(req.Context(), sessionWithOrg(orgID)))
}

func TestAuditTimelineMapsEvents(t *testing.T) {
	orgID := uuid.New()
	reader := &fakeAuditReader{events: []domain.SealedEvent{{
		Seq: 5,
		Event: domain.AuditEvent{
			Action:     domain.ActionMembershipRevoke,
			Outcome:    domain.Allowed,
			Actor:      domain.AuditActor{IdentitySubject: "subj-abc"},
			Target:     domain.AuditTarget{Type: "membership", ID: "m1", Label: "revoke"},
			OccurredAt: time.Unix(1000, 0),
			Context:    domain.AuditContext{PrivilegedCorrelationID: "pcid-1"},
		},
	}}}
	rr := httptest.NewRecorder()
	NewAuditTimelineHandler(reader).ServeHTTP(rr, auditTimelineReq(orgID, ""))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	if reader.gotOrg != orgID {
		t.Fatalf("read org %v, want session org %v", reader.gotOrg, orgID)
	}
	if reader.gotLimit != auditTimelineDefaultLimit {
		t.Fatalf("default limit = %d, want %d", reader.gotLimit, auditTimelineDefaultLimit)
	}
	var body struct {
		Events []auditEventItem `json:"events"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Events) != 1 {
		t.Fatalf("want 1 event, got %d", len(body.Events))
	}
	e := body.Events[0]
	if e.Seq != 5 || e.Action != "membership.revoke" || e.Outcome != "success" || e.ActorSubject != "subj-abc" {
		t.Fatalf("unexpected event: %+v", e)
	}
}

func TestAuditTimelineClampsLimit(t *testing.T) {
	reader := &fakeAuditReader{}
	rr := httptest.NewRecorder()
	NewAuditTimelineHandler(reader).ServeHTTP(rr, auditTimelineReq(uuid.New(), "?limit=999"))
	if reader.gotLimit != auditTimelineMaxLimit {
		t.Fatalf("limit = %d, want clamp to %d", reader.gotLimit, auditTimelineMaxLimit)
	}
}

func TestAuditTimelineFailsClosed(t *testing.T) {
	// No session.
	rr := httptest.NewRecorder()
	NewAuditTimelineHandler(&fakeAuditReader{}).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/audit/timeline", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("no session: status %d, want 401", rr.Code)
	}

	// Read error → 500 (never an empty timeline as authoritative).
	rr = httptest.NewRecorder()
	NewAuditTimelineHandler(&fakeAuditReader{err: errors.New("boom")}).ServeHTTP(rr, auditTimelineReq(uuid.New(), ""))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("read error: status %d, want 500", rr.Code)
	}
}
