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

// NotificationKind names the sort of notification, so a channel can route or
// format it. These are stable machine tokens, not display text.
type NotificationKind string

const (
	// NotifyDelegationStarted tells a target identity that an impersonation
	// session over it has begun (ADR-0008 §2 / pacote 004 T-005).
	NotifyDelegationStarted NotificationKind = "delegation.started"
	// NotifyDelegationRevoked tells the target that a delegation was revoked.
	NotifyDelegationRevoked NotificationKind = "delegation.revoked"
	// NotifyBreakglassRequested alerts a tenant's security channel that a
	// break-glass request was created (pacote 004 T-011), at REQUEST time.
	NotifyBreakglassRequested NotificationKind = "breakglass.requested"
)

// Notification is a message delivered to a recipient — the delegation target, or
// a tenant's security channel for break-glass. It carries NO personal data: the
// Recipient is an OPAQUE reference (an identity subject or a channel id) and
// Detail is a non-personal description. OrganizationID scopes the tenant.
type Notification struct {
	OrganizationID string
	Recipient      string
	Kind           NotificationKind
	Detail         string
}

// Notifier delivers a notification to a person or a channel. Unlike Alerter
// (best-effort operational paging), a Notifier's availability is a PRECONDITION
// for privileged flows: break-glass is FAIL-CLOSED on it — a request with no
// available notification channel is denied (spec "Canal indisponível", T-013).
// Notify returns an error when it cannot deliver.
type Notifier interface {
	// Notify delivers n, or returns an error if delivery failed.
	Notify(ctx context.Context, n Notification) error
	// Available reports whether at least one notification channel is configured
	// for the organization — the gate the break-glass flow checks BEFORE creating
	// a request, so an emergency access never proceeds without a way to alert.
	Available(ctx context.Context, organizationID string) bool
}
