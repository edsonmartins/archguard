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
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ErrInvalidReview is returned when constructing a post-use review with missing
// data or for a grant that does not require one.
var ErrInvalidReview = errors.New("post_use_review: dados obrigatórios ausentes")

// PostUseReview is the mandatory review artifact recorded after a break-glass
// grant is USED (ADR-0008 §3 / spec "Revisão pós-uso obrigatória"): a reviewer
// records their assessment of the emergency access. Until one exists for a
// terminated break-glass grant, the grant is a PENDING review — visible and
// escalated (spec "Revisão pendente").
type PostUseReview struct {
	ID                   uuid.UUID
	GrantID              uuid.UUID
	OrganizationID       uuid.UUID
	ReviewerMembershipID uuid.UUID
	// Notes is the reviewer's assessment — mandatory (a review that says nothing is
	// not a review). It is tenant content (may hold contextual data), not indexed
	// by person.
	Notes string
}

// NeedsReview reports whether a grant requires a post-use review: a break-glass
// grant that became active and has since ENDED (expired or revoked). A grant that
// was denied or rejected before ever activating was never used, so it needs no
// post-use review; a normal (non-break-glass) grant needs none either.
func (g PrivilegedGrant) NeedsReview() bool {
	if g.Origin != GrantBreakglass {
		return false
	}
	return g.Status == GrantExpired || g.Status == GrantRevoked
}

// NewPostUseReview records a reviewer's assessment of a used break-glass grant.
// It requires the grant to actually NEED a review (a terminated break-glass) and
// non-empty notes.
func NewPostUseReview(grant PrivilegedGrant, reviewerMembershipID uuid.UUID, notes string) (PostUseReview, error) {
	if !grant.NeedsReview() {
		return PostUseReview{}, fmt.Errorf("%w: a concessão não requer revisão pós-uso", ErrInvalidReview)
	}
	if reviewerMembershipID == uuid.Nil {
		return PostUseReview{}, fmt.Errorf("%w: revisor ausente", ErrInvalidReview)
	}
	if notes == "" {
		return PostUseReview{}, fmt.Errorf("%w: a revisão exige um parecer", ErrInvalidReview)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return PostUseReview{}, fmt.Errorf("post_use_review: geração de UUIDv7 falhou: %w", err)
	}
	return PostUseReview{
		ID:                   id,
		GrantID:              grant.ID,
		OrganizationID:       grant.OrganizationID,
		ReviewerMembershipID: reviewerMembershipID,
		Notes:                notes,
	}, nil
}
