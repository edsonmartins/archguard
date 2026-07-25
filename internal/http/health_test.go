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
	"testing"
)

type fakeChecker struct{ subs []Subsystem }

func (f fakeChecker) CheckHealth(context.Context) []Subsystem { return f.subs }

// TestAggregateStatusWorstWins pins the honest-aggregate rule (RFC-0005 §9): the
// overall status is never better than the worst subsystem, and an empty probe set
// is unavailable, never ok.
func TestAggregateStatusWorstWins(t *testing.T) {
	cases := []struct {
		name string
		subs []Subsystem
		want string
	}{
		{"empty is unavailable", nil, StatusUnavailable},
		{"all ok", []Subsystem{{Status: StatusOK}, {Status: StatusOK}}, StatusOK},
		{"one degraded", []Subsystem{{Status: StatusOK}, {Status: StatusDegraded}}, StatusDegraded},
		{"unavailable dominates degraded", []Subsystem{{Status: StatusDegraded}, {Status: StatusUnavailable}}, StatusUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := aggregateStatus(tc.subs); got != tc.want {
				t.Fatalf("aggregateStatus = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHealthHandlerReportsSubsystems(t *testing.T) {
	checker := fakeChecker{subs: []Subsystem{
		{Name: "database", Status: StatusOK},
		{Name: "custody", Status: StatusUnavailable, Detail: "sem cofre"},
	}}
	rr := httptest.NewRecorder()
	NewHealthHandler(checker).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	var body struct {
		Status     string      `json:"status"`
		Subsystems []Subsystem `json:"subsystems"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// A degraded/unavailable subsystem must drag the aggregate down — no false green.
	if body.Status != StatusUnavailable {
		t.Fatalf("aggregate = %q, want unavailable (a subsystem is down)", body.Status)
	}
	if len(body.Subsystems) != 2 {
		t.Fatalf("want 2 subsystems, got %d", len(body.Subsystems))
	}
}
