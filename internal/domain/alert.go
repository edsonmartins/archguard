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

package domain

import "context"

// Severity ranks an alert. SeverityCritical is the maximum — the level a
// tamper-detection divergence or an audit-unavailable condition must raise
// (RFC-0003 §6 / ADR-0013).
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Alert is a structured operational alert. It carries NO personal data — only
// pseudonymous references (an organization id, a seq) and a message — so it can
// flow to the observability pipeline without leaking (I-3.2).
type Alert struct {
	Severity Severity
	Subject  string
	Detail   string
}

// Alerter delivers an alert to the operations pipeline (paging / observability,
// ADR-0013, pacote 010). This port is the seam; the provisional implementation
// records alerts for dev/CI. An alert that cannot be delivered is itself an
// incident, but delivery is best-effort from the caller's point of view — the
// caller must not, for example, skip denying an operation because the alert
// failed.
type Alerter interface {
	Alert(ctx context.Context, alert Alert) error
}
