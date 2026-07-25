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

package boot

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func resetMux() {
	muxMu.Lock()
	apiMux = nil
	muxMu.Unlock()
}

// TestAPIHandlerNilBeforeInit guards the fail-closed contract: before InitAPIMux,
// APIHandler must return an explicit nil (not a typed-nil *ServeMux), so the
// bridge's `== nil` check fires and it answers 503 instead of open access.
func TestAPIHandlerNilBeforeInit(t *testing.T) {
	resetMux()
	if APIHandler() != nil {
		t.Fatalf("APIHandler() must be nil before InitAPIMux")
	}
}

func TestVersionEndpoint(t *testing.T) {
	resetMux()
	InitAPIMux()
	h := APIHandler()
	if h == nil {
		t.Fatalf("APIHandler() must be non-nil after InitAPIMux")
	}

	// Handlers are registered relative to APIBasePath (the bridge strips it), so
	// the version probe is served at "/version".
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/version", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /version: status %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("GET /version: content-type %q, want application/json", ct)
	}

	// Non-GET is rejected.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/version", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /version: status %d, want 405", rr.Code)
	}
}

func TestRegisterAPIHandlerReachable(t *testing.T) {
	resetMux()
	InitAPIMux()
	RegisterAPIHandler("/probe/mounted", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rr := httptest.NewRecorder()
	APIHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/probe/mounted", nil))
	if rr.Code != http.StatusTeapot {
		t.Fatalf("mounted handler: status %d, want 418", rr.Code)
	}

	// An unregistered path is a 404 (fail-closed default of the mux), never a hit.
	rr = httptest.NewRecorder()
	APIHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/probe/absent", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("absent path: status %d, want 404", rr.Code)
	}
}
