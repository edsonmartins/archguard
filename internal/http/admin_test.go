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
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAdminAllowsAdmin(t *testing.T) {
	served := false
	h := RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(WithAdmin(req.Context(), true))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !served || rr.Code != http.StatusOK {
		t.Fatalf("admin caller should pass: served=%v code=%d", served, rr.Code)
	}
}

// TestRequireAdminDeniesNonAdmin and the absent case are the fail-closed gate:
// only a caller the bridge marked admin gets through.
func TestRequireAdminDeniesNonAdmin(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  bool
		set  bool
	}{
		{"explicit non-admin", false, true},
		{"absent flag", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			served := false
			h := RequireAdmin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served = true }))
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if tc.set {
				req = req.WithContext(WithAdmin(req.Context(), tc.ctx))
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if served {
				t.Fatalf("non-admin must not reach the handler")
			}
			if rr.Code != http.StatusForbidden {
				t.Fatalf("status %d, want 403", rr.Code)
			}
		})
	}
}
