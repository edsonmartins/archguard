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

	"github.com/casdoor/casdoor/internal/domain"
)

// TestPipelineFailsClosedWithoutBinding is the core T-004 guarantee: a domain
// handler wrapped by the pipeline never runs when the request carries no session
// binding (the state until the login bridge, T-004b). The resolver short-circuits
// on the absent binding, so the DB-backed loader is never reached — the test needs
// no database (nil pool). Even a low L1 operation is denied without a session.
func TestPipelineFailsClosedWithoutBinding(t *testing.T) {
	p := NewPipeline(nil)
	if err := p.RegisterOperation(domain.Operation{
		ID:          "test.read",
		Level:       domain.L1,
		Description: "leitura de teste",
	}); err != nil {
		t.Fatalf("RegisterOperation: %v", err)
	}

	served := false
	wrapped := p.Require("test.read", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served = true
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	// No WithSessionBinding on the context => contextBinding.Bound returns ok=false.
	wrapped.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/test", nil))

	if served {
		t.Fatalf("handler must NOT run without a resolved session (fail-closed)")
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("response must be a denial, got 200")
	}
}

func TestActivePipelineSingleton(t *testing.T) {
	pipelineMu.Lock()
	activePipeline = nil
	pipelineMu.Unlock()

	if ActivePipeline() != nil {
		t.Fatalf("ActivePipeline() should be nil before InitPipeline")
	}
	InitPipeline(nil)
	if ActivePipeline() == nil {
		t.Fatalf("ActivePipeline() should be set after InitPipeline")
	}
}
