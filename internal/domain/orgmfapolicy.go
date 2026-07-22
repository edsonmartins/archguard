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

	"github.com/google/uuid"
)

// DefaultOrgMinimumAAL is the platform baseline an organization has before it
// sets an explicit MFA policy: a valid session (AAL1). It is a FLOOR the org may
// only RAISE — the per-operation levels (L1/L2/L3) still apply on top, so a
// baseline of AAL1 is not permissive, it just adds no tenant-wide constraint
// beyond what each operation already demands.
const DefaultOrgMinimumAAL = AAL1

// ErrInvalidOrgMFAPolicy is returned when building a policy with no organization
// or an undefined minimum level.
var ErrInvalidOrgMFAPolicy = errors.New("orgmfapolicy: organização e nível mínimo são obrigatórios")

// OrgMFAPolicy is one organization's tenant-wide authentication floor: the
// MINIMUM assurance level a session must prove to operate in that tenant (ADR-0010
// "Política de MFA por organização"). A stricter tenant demands more — MinimumAAL
// AAL2 means "MFA obrigatório" (a strong factor), AAL3 means "WebAuthn
// obrigatório" (phishing-resistant). It is the requirement the tenant switch
// (T-011) and the assurance guard compose with the per-operation level, always
// taking the MORE restrictive of the two.
type OrgMFAPolicy struct {
	OrganizationID uuid.UUID
	MinimumAAL     AAL
}

// NewOrgMFAPolicy builds a policy, refusing a nil organization or an undefined
// level.
func NewOrgMFAPolicy(organizationID uuid.UUID, minimum AAL) (OrgMFAPolicy, error) {
	if organizationID == uuid.Nil {
		return OrgMFAPolicy{}, fmt.Errorf("%w: organização nula", ErrInvalidOrgMFAPolicy)
	}
	if !minimum.Valid() {
		return OrgMFAPolicy{}, fmt.Errorf("%w: nível %q", ErrInvalidOrgMFAPolicy, minimum)
	}
	return OrgMFAPolicy{OrganizationID: organizationID, MinimumAAL: minimum}, nil
}

// DefaultOrgMFAPolicy is the policy of an organization that has not set one: the
// platform baseline floor.
func DefaultOrgMFAPolicy(organizationID uuid.UUID) OrgMFAPolicy {
	return OrgMFAPolicy{OrganizationID: organizationID, MinimumAAL: DefaultOrgMinimumAAL}
}

// RequiresPhishingResistant reports whether the policy forces a phishing-resistant
// factor (WebAuthn) — true exactly when the floor is AAL3.
func (p OrgMFAPolicy) RequiresPhishingResistant() bool {
	return p.MinimumAAL == AAL3
}

// SatisfiedBy reports whether a session that proved provenAAL meets this tenant's
// floor. Fail-closed: an undefined proven level satisfies nothing.
func (p OrgMFAPolicy) SatisfiedBy(provenAAL AAL) bool {
	return provenAAL.AtLeast(p.MinimumAAL)
}

// OrgMFAPolicyStore persists per-organization MFA policies. Get returns the
// organization's effective policy — the stored one, or DefaultOrgMFAPolicy when
// none is set (an unset policy is a decision: the baseline floor, not an error).
// A STORE FAILURE, by contrast, is an error the caller denies on (INV-6): the
// implementation must never paper over an unreadable policy with a default. Set
// upserts the policy (tenant admin, an L3 operation).
type OrgMFAPolicyStore interface {
	Get(ctx context.Context, organizationID uuid.UUID) (OrgMFAPolicy, error)
	Set(ctx context.Context, policy OrgMFAPolicy) error
}
