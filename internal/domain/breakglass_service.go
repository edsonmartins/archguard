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

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrNoNotificationChannel is returned when a break-glass request cannot proceed
// because the tenant has no available notification channel — fail-closed (spec
// "Canal indisponível", T-013): emergency access never happens silently.
var ErrNoNotificationChannel = errors.New("breakglass: nenhum canal de notificação disponível — solicitação negada")

// BreakglassRequester opens break-glass requests, emitting the real-time alert at
// REQUEST time (T-011) and enforcing the notification-channel fail-closed
// precondition (T-013). It is a domain service over the Notifier port; the audit
// fail-closed leg (T-013) and persistence are wired by the adapter that composes
// this with the audit writer and the grant store.
type BreakglassRequester struct {
	notifier Notifier
}

// NewBreakglassRequester builds the requester over a notifier.
func NewBreakglassRequester(notifier Notifier) *BreakglassRequester {
	return &BreakglassRequester{notifier: notifier}
}

// Request opens a break-glass grant for the subject over the target. It is
// FAIL-CLOSED on the notification channel: if no channel is available for the
// organization the request is DENIED (ErrNoNotificationChannel) before any grant
// exists. On success it creates the requested grant and emits the real-time alert
// IMMEDIATELY — before any approval (spec "Alerta na solicitação"). If the alert
// cannot be delivered the request fails (the grant is not returned), so a
// break-glass never proceeds unannounced.
func (r *BreakglassRequester) Request(ctx context.Context, organizationID, subjectMembershipID uuid.UUID, target GrantTarget, policy BreakglassPolicy, justification, incidentRef string, notBefore, expiresAt time.Time) (PrivilegedGrant, error) {
	// Fail-closed precondition: a channel must exist to alert on BEFORE we accept
	// the emergency access (T-013).
	if !r.notifier.Available(ctx, organizationID.String()) {
		return PrivilegedGrant{}, ErrNoNotificationChannel
	}
	g, err := NewBreakglassRequest(organizationID, subjectMembershipID, target, policy.RequiredApprovals, justification, incidentRef, notBefore, expiresAt)
	if err != nil {
		return PrivilegedGrant{}, err
	}
	// Real-time alert at request time — before approvals.
	if err := r.notifier.Notify(ctx, g.RequestedNotification()); err != nil {
		return PrivilegedGrant{}, fmt.Errorf("breakglass: alerta de solicitação falhou: %w", err)
	}
	return g, nil
}
