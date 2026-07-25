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
	"net/http"
)

// Subsystem status values. Ordered by severity so the handler can aggregate
// "the worst wins" (RFC-0005 §9: an aggregate must never hide a negative).
const (
	StatusOK          = "ok"
	StatusDegraded    = "degraded"
	StatusUnavailable = "unavailable"
)

// Subsystem is the health of one dependency (database, custody, …). Detail is
// operational, non-secret context.
type Subsystem struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// HealthChecker probes the subsystems the control plane depends on. The composition
// root implements it (it holds the pool and the adapter factory).
type HealthChecker interface {
	CheckHealth(ctx context.Context) []Subsystem
}

// HealthHandler serves GET /health: the status of each subsystem plus an honest
// aggregate — the worst subsystem status wins, so a green overall can never coexist
// with a degraded dependency in the detail (RFC-0005 §9). Thin (CLAUDE.md §6).
type HealthHandler struct {
	checker HealthChecker
}

// NewHealthHandler builds the handler over a checker.
func NewHealthHandler(checker HealthChecker) *HealthHandler {
	return &HealthHandler{checker: checker}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	subsystems := h.checker.CheckHealth(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     aggregateStatus(subsystems),
		"subsystems": subsystems,
	})
}

// aggregateStatus returns the worst status among the subsystems — the honest
// aggregate. An empty set is unavailable (we could not probe anything), never ok.
func aggregateStatus(subsystems []Subsystem) string {
	if len(subsystems) == 0 {
		return StatusUnavailable
	}
	worst := StatusOK
	for _, s := range subsystems {
		if severity(s.Status) > severity(worst) {
			worst = s.Status
		}
	}
	return worst
}

func severity(status string) int {
	switch status {
	case StatusUnavailable:
		return 2
	case StatusDegraded:
		return 1
	default:
		return 0
	}
}
