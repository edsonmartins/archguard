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

// Package tenantswitch holds provisional implementations of the tenant-switch
// ports (domain.TenantAuthPolicy / domain.SessionAuditor).
//
// These are NOT the production controls: the per-organization MFA/step-up
// policy is pacote 005 and the tamper-evident append-only audit trail is pacote
// 003. They exist so the TenantSwitcher is usable and testable now, WITHOUT
// ever failing open (INV-6): outside the dev profile the policy demands the
// STRONGEST level, and the auditor is explicit about durability.
package tenantswitch

import (
	"context"
	"fmt"
	"sync"

	"github.com/casdoor/casdoor/internal/deploy"
	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// ProfilePolicy is the provisional destination policy: under the dev profile it
// requires AAL1 (any authenticated session may switch); under pilot/production
// it requires AAL3 — the STRICTEST level — until the real per-organization
// policy (pacote 005) is wired. Fail-closed by design: the provisional default
// can only over-demand, never under-demand.
type ProfilePolicy struct{}

// NewProfilePolicy builds the provisional profile-gated policy.
func NewProfilePolicy() *ProfilePolicy { return &ProfilePolicy{} }

// RequiredAAL answers the destination's requirement per the active profile. A
// nil organization is a caller bug and is refused (the caller denies, INV-6).
func (p *ProfilePolicy) RequiredAAL(_ context.Context, organizationID uuid.UUID) (domain.AAL, error) {
	if organizationID == uuid.Nil {
		return "", fmt.Errorf("tenantswitch: organização de destino nula")
	}
	if deploy.Active() == deploy.Dev {
		return domain.AAL1, nil
	}
	return domain.AAL3, nil
}

var _ domain.TenantAuthPolicy = (*ProfilePolicy)(nil)

// MemorySwitchAuditor is a PROVISIONAL, dev-only audit sink: it appends each
// switch event to an in-memory slice. It is NOT durable — records are lost on
// restart — and NOT tamper-evident, so it is not the production audit trail
// (pacote 003). In pilot/production the real append-only trail MUST be wired in
// its place.
type MemorySwitchAuditor struct {
	mu     sync.Mutex
	events []domain.TenantSwitchEvent
}

// NewMemorySwitchAuditor builds an empty provisional in-memory auditor.
func NewMemorySwitchAuditor() *MemorySwitchAuditor { return &MemorySwitchAuditor{} }

// RecordTenantSwitch appends the event. It validates first so a malformed event
// is not "recorded" as if it were a real one.
func (m *MemorySwitchAuditor) RecordTenantSwitch(_ context.Context, event domain.TenantSwitchEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

// Events returns a copy of what was recorded, for inspection in tests.
func (m *MemorySwitchAuditor) Events() []domain.TenantSwitchEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.TenantSwitchEvent, len(m.events))
	copy(out, m.events)
	return out
}

var _ domain.SessionAuditor = (*MemorySwitchAuditor)(nil)
