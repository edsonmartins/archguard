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

	"github.com/casdoor/casdoor/internal/deploy"
)

// TestMountAuditVerifyIsMountedAndFailsClosed proves the T-005 contract: after
// MountCapabilities the audit-verify endpoint is reachable (not 404) but, being
// L3, is denied for a request with no session (fail-closed) — the handler behind
// it never runs. Needs no database: the L3 gate rejects before the verifier.
func TestMountAuditVerifyIsMountedAndFailsClosed(t *testing.T) {
	resetMux()
	InitAPIMux()
	InitPipeline(nil)
	InitFactory(deploy.Dev, nil, nil)

	if err := MountCapabilities(); err != nil {
		t.Fatalf("MountCapabilities: %v", err)
	}

	rr := httptest.NewRecorder()
	// No session binding on the context => the L3 assurance gate must deny.
	APIHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/audit/verify?organization_id=x", nil))

	if rr.Code == http.StatusNotFound {
		t.Fatalf("audit/verify should be MOUNTED (got 404 — not registered)")
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("audit/verify is L3 and must be denied without an L3 session (got 200)")
	}
}

// TestMountSessionIsMountedAndFailsClosed: /session is reachable (not 404) but
// requires an authenticated session — an unbound request never returns 200.
func TestMountSessionIsMountedAndFailsClosed(t *testing.T) {
	resetMux()
	InitAPIMux()
	InitPipeline(nil)
	InitFactory(deploy.Dev, nil, nil)
	if err := MountCapabilities(); err != nil {
		t.Fatalf("MountCapabilities: %v", err)
	}

	rr := httptest.NewRecorder()
	APIHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/session", nil))

	if rr.Code == http.StatusNotFound {
		t.Fatalf("/session should be MOUNTED (got 404)")
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("/session must not return 200 without an authenticated session")
	}
}

// TestMountTenantsIsMountedAndFailsClosed: /tenants is reachable (not 404) but
// requires an authenticated session — an unbound request never returns 200.
func TestMountTenantsIsMountedAndFailsClosed(t *testing.T) {
	resetMux()
	InitAPIMux()
	InitPipeline(nil)
	InitFactory(deploy.Dev, nil, nil)
	if err := MountCapabilities(); err != nil {
		t.Fatalf("MountCapabilities: %v", err)
	}

	rr := httptest.NewRecorder()
	APIHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/tenants", nil))

	if rr.Code == http.StatusNotFound {
		t.Fatalf("/tenants should be MOUNTED (got 404)")
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("/tenants must not return 200 without an authenticated session")
	}
}

// TestMountMembershipsIsMountedAndAdminGated: /memberships is reachable (not 404)
// but an unauthenticated, non-admin request never returns 200 (assurance + admin
// gate both fail closed).
func TestMountMembershipsIsMountedAndAdminGated(t *testing.T) {
	resetMux()
	InitAPIMux()
	InitPipeline(nil)
	InitFactory(deploy.Dev, nil, nil)
	if err := MountCapabilities(); err != nil {
		t.Fatalf("MountCapabilities: %v", err)
	}

	rr := httptest.NewRecorder()
	APIHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/memberships", nil))

	if rr.Code == http.StatusNotFound {
		t.Fatalf("/memberships should be MOUNTED (got 404)")
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("/memberships must not return 200 without an authenticated admin session")
	}
}

// TestMountHealthIsMountedAndAdminGated: /health is reachable (not 404) but
// admin-gated — an unauthenticated request never returns 200.
func TestMountHealthIsMountedAndAdminGated(t *testing.T) {
	resetMux()
	InitAPIMux()
	InitPipeline(nil)
	InitFactory(deploy.Dev, nil, nil)
	if err := MountCapabilities(); err != nil {
		t.Fatalf("MountCapabilities: %v", err)
	}

	rr := httptest.NewRecorder()
	APIHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rr.Code == http.StatusNotFound {
		t.Fatalf("/health should be MOUNTED (got 404)")
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("/health must not return 200 without an authenticated admin session")
	}
}

// TestMountGrantsIsMountedAndAdminGated: /grants is reachable (not 404) but
// admin-gated — an unauthenticated request never returns 200.
func TestMountGrantsIsMountedAndAdminGated(t *testing.T) {
	resetMux()
	InitAPIMux()
	InitPipeline(nil)
	InitFactory(deploy.Dev, nil, nil)
	if err := MountCapabilities(); err != nil {
		t.Fatalf("MountCapabilities: %v", err)
	}

	rr := httptest.NewRecorder()
	APIHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/grants", nil))

	if rr.Code == http.StatusNotFound {
		t.Fatalf("/grants should be MOUNTED (got 404)")
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("/grants must not return 200 without an authenticated admin session")
	}
}

// TestMountAccessReviewIsMountedAndAdminGated: /access/effective is reachable
// (not 404) but admin-gated — an unauthenticated request never returns 200.
func TestMountAccessReviewIsMountedAndAdminGated(t *testing.T) {
	resetMux()
	InitAPIMux()
	InitPipeline(nil)
	InitFactory(deploy.Dev, nil, nil)
	if err := MountCapabilities(); err != nil {
		t.Fatalf("MountCapabilities: %v", err)
	}

	rr := httptest.NewRecorder()
	APIHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/access/effective?membership=x&asset=y", nil))

	if rr.Code == http.StatusNotFound {
		t.Fatalf("/access/effective should be MOUNTED (got 404)")
	}
	if rr.Code == http.StatusOK {
		t.Fatalf("/access/effective must not return 200 without an authenticated admin session")
	}
}

// TestMountCapabilitiesRequiresInit fails closed when the pipeline/factory are not
// initialized, rather than mounting an unguarded surface.
func TestMountCapabilitiesRequiresInit(t *testing.T) {
	pipelineMu.Lock()
	activePipeline = nil
	pipelineMu.Unlock()

	if err := MountCapabilities(); err == nil {
		t.Fatalf("MountCapabilities should error when the pipeline is not initialized")
	}
}
