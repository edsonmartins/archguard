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

// Package notification adapts the domain.Notifier port to the tenant's configured
// notification providers (the curated set — webhook/Slack/Teams/Google Chat/Custom
// HTTP, ADR-0015 §3). It is the fail-closed alert channel for break-glass: an emergency
// access is denied unless the tenant has a channel to announce it on, and the alert must
// actually be delivered (ADR-0008 / RFC-0004 §4 — the notify is a remote call outside any
// DB transaction). Reads the legacy Provider config via `object` (allowed in an adapter;
// INV-3 constrains only the domain).
package notification

import (
	"context"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/casdoor/casdoor/object"
)

// ProviderNotifier is the concrete domain.Notifier over the tenant's notification
// providers.
type ProviderNotifier struct{}

// NewProviderNotifier builds the notifier.
func NewProviderNotifier() *ProviderNotifier { return &ProviderNotifier{} }

var _ domain.Notifier = (*ProviderNotifier)(nil)

// tenantProviders returns the organization's OWN notification providers. Casdoor's
// GetProvidersByCategory also returns the global "admin" providers; those are excluded so
// a tenant with no channel of its own is correctly reported unavailable (fail-closed: we
// never rely on a platform-global channel to announce a tenant's emergency access).
func tenantProviders(organizationID string) []*object.Provider {
	all, err := object.GetProvidersByCategory(organizationID, "Notification")
	if err != nil {
		return nil
	}
	own := make([]*object.Provider, 0, len(all))
	for _, p := range all {
		if p != nil && p.Owner == organizationID {
			own = append(own, p)
		}
	}
	return own
}

// Available reports whether the organization has at least one notification channel.
func (n *ProviderNotifier) Available(_ context.Context, organizationID string) bool {
	return len(tenantProviders(organizationID)) > 0
}

// Notify delivers the notification through the organization's channels. It returns nil if
// AT LEAST ONE channel accepted the message, and an error only if every channel failed (or
// none exists) — fail-closed: an undeliverable break-glass alert fails the request, so the
// emergency access never proceeds unannounced.
func (n *ProviderNotifier) Notify(_ context.Context, notif domain.Notification) error {
	providers := tenantProviders(notif.OrganizationID)
	if len(providers) == 0 {
		return domain.ErrNoNotificationChannel
	}
	var lastErr error
	delivered := false
	for _, p := range providers {
		if err := object.SendNotification(p, notif.Detail, notif.Recipient); err != nil {
			lastErr = err
			continue
		}
		delivered = true
	}
	if delivered {
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return domain.ErrNoNotificationChannel
}
