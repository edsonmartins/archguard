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

package postgres

import (
	"context"

	"github.com/casdoor/casdoor/internal/domain"
)

// enqueueGrantProjection projects a grant to its `has_active_grant` tuple and enqueues
// the update to the AuthzOutbox IN THE SAME transaction (M4, T-027, RFC-0004 §4). It is
// called at every grant transition (create/approve/revoke/step-up/expire): ProjectGrant
// writes the conditioned tuple when the grant is active and deletes it otherwise, so the
// projection follows the lifecycle. It is a NO-OP when the target does not name a
// registered asset (opaque broker target) — fail-safe, never blocks the transition.
//
// Idempotent: re-enqueuing the same write/delete is a no-op at the publisher, so calling
// it on a transition that does not change activity is harmless.
func enqueueGrantProjection(ctx context.Context, ttx *TenantTx, g domain.PrivilegedGrant) error {
	assetRef, ok := g.Target.AssetRef(g.OrganizationID)
	if !ok {
		return nil
	}
	update, err := domain.ProjectGrant(g, assetRef)
	if err != nil {
		return err
	}
	return NewAuthzOutbox(ttx.Tx()).Enqueue(ctx, []domain.TupleUpdate{update})
}
