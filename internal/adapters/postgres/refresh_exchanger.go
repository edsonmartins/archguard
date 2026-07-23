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
	"fmt"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
)

// RefreshExchanger performs the refresh-token exchange with mandatory rotation
// and reuse detection (pacote 006 T-007/T-008). On a normal exchange it rotates
// (old → rotated, successor active) atomically. On REUSE — a rotated/revoked
// token presented again — it revokes the WHOLE family, writes a high-severity
// audit event and raises a critical alert, all in one transaction, then denies.
type RefreshExchanger struct {
	repo    *TenantRepository
	audit   AuditEmitter
	alerter domain.Alerter
}

// NewRefreshExchanger builds the exchanger over the tenant repository (the org
// the token belongs to, resolved by the token endpoint), the audit emitter and
// the alerter.
func NewRefreshExchanger(repo *TenantRepository, audit AuditEmitter, alerter domain.Alerter) *RefreshExchanger {
	return &RefreshExchanger{repo: repo, audit: audit, alerter: alerter}
}

// ExchangeResult is a successful rotation: the new secret to hand the client and
// the new token record.
type ExchangeResult struct {
	NewSecret string
	NewToken  domain.RefreshToken
}

// Exchange validates a presented refresh secret and rotates it. It returns
// ErrRefreshTokenNotFound for an unknown token, domain.ErrRefreshReuse when reuse
// is detected (after revoking the family, auditing and alerting), or the new
// secret on success. newExpiry is the successor's expiry. Everything happens in
// ONE transaction with the row locked FOR UPDATE, so a concurrent double-exchange
// cannot both succeed.
func (x *RefreshExchanger) Exchange(ctx context.Context, presentedSecret string, newExpiry time.Time) (ExchangeResult, error) {
	var result ExchangeResult
	var reuse bool
	err := x.repo.WithTenantTx(ctx, func(ttx *TenantTx) error {
		store := NewRefreshTokenStore(ttx)
		tok, err := store.GetByHash(ctx, domain.HashRefreshToken(presentedSecret))
		if err != nil {
			return err
		}

		// Reuse detection: a rotated/revoked token presented again kills the family.
		// The revocation and its audit MUST COMMIT, so we do not return the reuse
		// error here (that would roll back the very revocation we need). We commit
		// the family revocation + audit + alert, then signal reuse after the tx.
		if reuseErr := tok.CheckReuse(); reuseErr != nil {
			if _, err := store.RevokeFamily(ctx, tok.FamilyID); err != nil {
				return err
			}
			if err := x.auditReuse(ctx, ttx, tok); err != nil {
				return err
			}
			x.alertReuse(ctx, tok)
			reuse = true
			return nil
		}

		if !tok.Usable(x.nowFromExpiry(newExpiry)) {
			// Active but expired: a normal expiry, not reuse. Deny without killing
			// the family.
			return fmt.Errorf("refresh: token expirado")
		}

		// Normal rotation.
		newSecret, newHash, err := domain.NewRefreshSecret()
		if err != nil {
			return err
		}
		successor, err := tok.Rotate(newHash, newExpiry)
		if err != nil {
			return err
		}
		if err := store.SetStatus(ctx, tok.ID, domain.RefreshRotated); err != nil {
			return err
		}
		if err := store.Create(ctx, successor); err != nil {
			return err
		}
		result = ExchangeResult{NewSecret: newSecret, NewToken: successor}
		return nil
	})
	if err != nil {
		return ExchangeResult{}, err
	}
	if reuse {
		// The family revocation, audit and alert have committed; now deny.
		return ExchangeResult{}, domain.ErrRefreshReuse
	}
	return result, nil
}

// nowFromExpiry derives a conservative "now" for the usability check from the
// successor expiry: the caller sets newExpiry = now + refresh TTL, so now is
// before it; using a moment just before newExpiry keeps an unexpired active token
// usable while still rejecting one already past its OWN expiry.
func (x *RefreshExchanger) nowFromExpiry(newExpiry time.Time) time.Time {
	return newExpiry.Add(-time.Nanosecond)
}

// auditReuse writes the high-severity reuse event (system actor — a stolen token
// has no legitimate principal) atomically in the exchange transaction.
func (x *RefreshExchanger) auditReuse(ctx context.Context, ttx *TenantTx, tok domain.RefreshToken) error {
	if x.audit == nil {
		return nil
	}
	_, err := x.audit.AppendTx(ctx, ttx.Tx(), domain.AuditEventInput{
		OrganizationID: tok.OrganizationID,
		Action:         domain.ActionRefreshReuse,
		Actor:          domain.AuditActor{IdentitySubject: "system"},
		Outcome:        domain.Failed,
		Target:         domain.AuditTarget{Type: "refresh_family", ID: tok.FamilyID.String(), Label: "reuso de refresh token"},
		Reason:         "reuso de refresh token detectado — família revogada",
	})
	return err
}

// alertReuse raises a critical operational alert. Delivery is best-effort — the
// family is already revoked and the event audited; a failed alert does not undo
// the (correct) denial.
func (x *RefreshExchanger) alertReuse(ctx context.Context, tok domain.RefreshToken) {
	if x.alerter == nil {
		return
	}
	_ = x.alerter.Alert(ctx, domain.Alert{
		Severity: domain.SeverityCritical,
		Subject:  "refresh_token.reuse",
		Detail:   "reuso de refresh token na família " + tok.FamilyID.String() + " — família revogada",
	})
}
