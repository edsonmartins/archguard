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
	"sync"
)

// APIBasePath is the versioned public prefix of the ArchGuard control-plane API.
// The Beego bridge (controllers) strips it before delegating, so handlers are
// registered relative to it (e.g. "/audit/verify" is served at "/api/v1/audit/verify").
const APIBasePath = "/api/v1"

// The API mux is the single net/http router the Beego bridge delegates to. It is
// built once at boot (InitAPIMux) and populated by the mount tasks (T-005+) via
// RegisterAPIHandler, before the server starts serving.
var (
	muxMu  sync.Mutex
	apiMux *http.ServeMux
)

// InitAPIMux creates the API mux with its built-in endpoints. It must run at boot
// before any RegisterAPIHandler call. Calling it again replaces the mux (and thus
// clears registrations), so it is invoked exactly once from main.
func InitAPIMux() {
	muxMu.Lock()
	defer muxMu.Unlock()
	apiMux = http.NewServeMux()
	apiMux.HandleFunc("/version", handleVersion)
}

// RegisterAPIHandler mounts a capability handler at the given path, relative to
// APIBasePath. Called at boot by the mount tasks. It is a boot-time programming
// error to call this before InitAPIMux; the mux is created defensively if so.
func RegisterAPIHandler(pattern string, h http.Handler) {
	muxMu.Lock()
	defer muxMu.Unlock()
	if apiMux == nil {
		apiMux = http.NewServeMux()
	}
	apiMux.Handle(pattern, h)
}

// APIHandler returns the composed API mux for the bridge to delegate to, or nil
// when InitAPIMux has not run. The bridge treats a nil handler as fail-closed
// (503), never as open access. Returning an explicit nil (not a typed-nil
// *ServeMux) so the caller's `== nil` check behaves.
func APIHandler() http.Handler {
	muxMu.Lock()
	defer muxMu.Unlock()
	if apiMux == nil {
		return nil
	}
	return apiMux
}

// handleVersion is the built-in liveness/version probe of the control-plane API.
// It reports the public API major version so the generated client (008 T-002) can
// assert compatibility.
func handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"apiVersion":"v1"}`))
}
