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

	"github.com/google/uuid"
)

func TestContextBindingRoundTrip(t *testing.T) {
	id := uuid.New()
	sid := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(WithSessionBinding(req.Context(), id, sid))

	gotID, gotSID, ok := contextBinding{}.Bound(req)
	if !ok {
		t.Fatalf("Bound should report ok=true when a binding is present")
	}
	if gotID != id || gotSID != sid {
		t.Fatalf("Bound = (%v,%v), want (%v,%v)", gotID, gotSID, id, sid)
	}
}

func TestContextBindingAbsentFailsClosed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	id, sid, ok := contextBinding{}.Bound(req)
	if ok {
		t.Fatalf("Bound should report ok=false with no binding (fail-closed)")
	}
	if id != uuid.Nil || sid != uuid.Nil {
		t.Fatalf("Bound should return Nil ids when absent, got (%v,%v)", id, sid)
	}
}
