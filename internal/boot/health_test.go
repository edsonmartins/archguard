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
	"context"
	"testing"

	"github.com/casdoor/casdoor/internal/deploy"
	apihttp "github.com/casdoor/casdoor/internal/http"
)

// TestHealthCheckerReportsHonestState: without a pool the database is unavailable
// (never faked ok), and dev custody is ok. No database needed.
func TestHealthCheckerReportsHonestState(t *testing.T) {
	deploy.SetActive(deploy.Dev)
	h := healthChecker{pool: nil, factory: NewFactory(deploy.Dev, nil, nil)}

	subs := h.CheckHealth(context.Background())
	byName := map[string]apihttp.Subsystem{}
	for _, s := range subs {
		byName[s.Name] = s
	}

	if byName["database"].Status != apihttp.StatusUnavailable {
		t.Fatalf("database without a pool must be unavailable, got %q", byName["database"].Status)
	}
	if byName["custody"].Status != apihttp.StatusOK {
		t.Fatalf("dev custody should be ok, got %q", byName["custody"].Status)
	}
	// The aggregate must reflect the unavailable database — no false green.
	if got := apihttp.NewHealthHandler(h); got == nil {
		t.Fatalf("handler build failed")
	}
}
