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
	"sort"
	"time"

	"github.com/google/uuid"
)

// AuditSchemaVersion is the version of the audit-event content format
// (RFC-0003 §2, schema_version). It is part of the canonicalized content, so a
// verifier knows which rules to recompute a historical event under. BUMP it
// only with a documented format change AND a fixed-vector test for the new
// version — a silent change invalidates every historical proof.
const AuditSchemaVersion = 1

// AssuranceLevel is the operation assurance level an action requires
// (ADR-0010 / INV-8: every API operation declares L1/L2/L3). It travels with
// each catalogued action so the auditor and the step-up policy agree on what a
// verb costs.
type AssuranceLevel string

const (
	// L1: read own profile, list sessions — a valid session suffices.
	L1 AssuranceLevel = "L1"
	// L2: change data, manage tenant users — a strong factor within a freshness
	// window.
	L2 AssuranceLevel = "L2"
	// L3: open a privileged session, approve break-glass, rotate a key, export
	// the audit trail — immediate phishing-resistant re-authentication.
	L3 AssuranceLevel = "L3"
)

// Action is a canonical audit verb (RFC-0003 §2). The catalog is CLOSED: an
// event cannot be created for an action that is not registered here, so a
// mis-typed or ad-hoc verb never reaches the trail and reporting-by-action is
// exact. Adding a verb is a deliberate edit to actionCatalog, with its
// assurance level.
type Action string

const (
	ActionAuthLogin             Action = "auth.login"
	ActionAuthLoginDenied       Action = "auth.login.denied"
	ActionAuthLogout            Action = "auth.logout"
	ActionTenantSelect          Action = "tenant.select"
	ActionTenantSwitch          Action = "tenant.switch"
	ActionMembershipInvite      Action = "membership.invite"
	ActionMembershipAccept      Action = "membership.accept"
	ActionMembershipRevoke      Action = "membership.revoke"
	ActionIdentitySuspend       Action = "identity.suspend"
	ActionIdentityDeprovision   Action = "identity.deprovision"
	ActionPrivilegedSessionOpen Action = "privileged.session.open"
	ActionBreakglassRequest     Action = "breakglass.request"
	ActionBreakglassApprove     Action = "breakglass.approve"
	ActionAdminMutation         Action = "admin.mutation"
	ActionKeyRotate             Action = "key.rotate"
	ActionAuditExport           Action = "audit.export"
	ActionAuditVerify           Action = "audit.verify"
	// MFA / step-up events (pacote 005). Enrollment and step-up are authentication
	// events; removing a strong factor is privileged (L3, spec "reset silencioso");
	// the peer-approved recovery and its reset are L3; lockout and stuffing alerts
	// are security-monitoring events.
	ActionFactorEnroll    Action = "factor.enroll"
	ActionFactorRemove    Action = "factor.remove"
	ActionAuthStepUp      Action = "auth.stepup"
	ActionAuthLockout     Action = "auth.lockout"
	ActionAuthStuffing    Action = "auth.stuffing_alert"
	ActionRecoveryRequest Action = "recovery.request"
	ActionRecoveryApprove Action = "recovery.approve"
	ActionRecoveryReset   Action = "recovery.reset"
)

// actionCatalog is the closed set of canonical actions and the assurance level
// each carries. Keep it sorted by verb for readability; CatalogedActions()
// returns a deterministic order regardless.
var actionCatalog = map[Action]AssuranceLevel{
	ActionAuthLogin:             L1,
	ActionAuthLoginDenied:       L1,
	ActionAuthLogout:            L1,
	ActionTenantSelect:          L1,
	ActionTenantSwitch:          L2,
	ActionMembershipInvite:      L2,
	ActionMembershipAccept:      L1,
	ActionMembershipRevoke:      L2,
	ActionIdentitySuspend:       L2,
	ActionIdentityDeprovision:   L3,
	ActionPrivilegedSessionOpen: L3,
	ActionBreakglassRequest:     L3,
	ActionBreakglassApprove:     L3,
	ActionAdminMutation:         L2,
	ActionKeyRotate:             L3,
	ActionAuditExport:           L3,
	ActionAuditVerify:           L3,
	ActionFactorEnroll:          L2,
	ActionFactorRemove:          L3,
	ActionAuthStepUp:            L1,
	ActionAuthLockout:           L1,
	ActionAuthStuffing:          L1,
	ActionRecoveryRequest:       L2,
	ActionRecoveryApprove:       L3,
	ActionRecoveryReset:         L3,
}

// Valid reports whether a is a registered canonical action.
func (a Action) Valid() bool {
	_, ok := actionCatalog[a]
	return ok
}

// AssuranceLevel returns the level the action requires, or "" if the action is
// not catalogued (callers should gate on Valid first).
func (a Action) AssuranceLevel() AssuranceLevel {
	return actionCatalog[a]
}

// CatalogedActions returns every registered action, sorted — a stable listing
// for tests, documentation and the console's filter.
func CatalogedActions() []Action {
	out := make([]Action, 0, len(actionCatalog))
	for a := range actionCatalog {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Errors of audit-event construction.
var (
	// ErrUnknownAction is returned when an event is created for an action that
	// is not in the closed catalog.
	ErrUnknownAction = errors.New("audit: ação não está no catálogo canônico")
	// ErrInvalidAuditEvent is returned for a missing organization, an actor with
	// no subject, or an undefined outcome.
	ErrInvalidAuditEvent = errors.New("audit: evento inválido")
)

// AuditActor is who performed the action (RFC-0003 §2). IdentitySubject is the
// opaque, stable pseudonym (identity.Subject) — NEVER an e-mail or name, so the
// actor field carries no plaintext personal data. MembershipID/SessionID locate
// the tenant context and session; Act names the real actor in a delegation or
// impersonation (RFC 8693 / ADR-0008), nil otherwise.
type AuditActor struct {
	IdentitySubject string
	MembershipID    *uuid.UUID
	SessionID       *uuid.UUID
	Act             *AuditActor
}

// AuditTarget is the resource the action affected (RFC-0003 §2). ID is an
// opaque identifier or pseudonym; Label is a short non-personal description.
type AuditTarget struct {
	Type  string
	ID    string
	Label string
}

// AuditContext is the origin/correlation context (RFC-0003 §2). IP and
// UserAgent are origin context; their LGPD classification is applied on the
// storage column (T-005), consistent with the other packages. AuthContextClass
// and AuthMethods carry the acr/amr of the session; TraceID correlates with
// telemetry (ADR-0013); PrivilegedCorrelationID unites the ArchGuard trail with
// the privileged-access components (pcid).
type AuditContext struct {
	IP                      string
	UserAgent               string
	AuthContextClass        string
	AuthMethods             []string
	TraceID                 string
	PrivilegedCorrelationID string
}

// AuditEvent is the CANONICAL CONTENT of one audit event — exactly the fields
// that get hashed (RFC-0003 §3: canonical(e) excludes the chain fields). The
// chain fields (seq, prev_hash, hash) are assigned by the write path and live
// in SealedEvent, so this type has nothing to accidentally include in its own
// hash. occurred_at is stamped by the write layer (a trusted time source,
// RFC-0003 §2), keeping the domain free of the wall clock and deterministic in
// tests.
type AuditEvent struct {
	SchemaVersion  int
	EventID        uuid.UUID
	OrganizationID uuid.UUID
	Action         Action
	Actor          AuditActor
	Target         AuditTarget
	// Outcome is the domain primitive (Allowed/Denied/Failed); the serialized
	// form success|denied|error is produced by SerializedOutcome for the wire
	// and the canonical content.
	Outcome Outcome
	Reason  string
	Context AuditContext
	// OccurredAt is the trusted-time-source timestamp of the event (RFC-0003
	// §2). It is stamped by the write path (T-004) from a trusted clock, NOT by
	// the domain constructor, so the domain stays free of the wall clock; it is
	// part of the canonical content (tampering with the time must be
	// detectable). Canonicalization truncates it to microseconds — the storage
	// precision — so a verifier reproduces the exact bytes from the stored row.
	OccurredAt time.Time
}

// SerializedOutcome maps the domain outcome to the RFC-0003 §2 wire form:
// Allowed→success, Denied→denied, Failed→error. The failure/denial distinction
// (INV-6) is preserved on the trail.
func (e AuditEvent) SerializedOutcome() string {
	switch e.Outcome {
	case Allowed:
		return "success"
	case Denied:
		return "denied"
	case Failed:
		return "error"
	default:
		return "unknown"
	}
}

// ParseOutcome maps the serialized wire form back to the domain outcome — the
// inverse of SerializedOutcome, used by the verifier when it reconstructs an
// event from its stored columns.
func ParseOutcome(s string) (Outcome, error) {
	switch s {
	case "success":
		return Allowed, nil
	case "denied":
		return Denied, nil
	case "error":
		return Failed, nil
	default:
		return Allowed, fmt.Errorf("%w: outcome serializado %q", ErrInvalidAuditEvent, s)
	}
}

// AuditEventInput carries the caller-supplied content of an event. NewAuditEvent
// validates it and mints the event id and schema version.
type AuditEventInput struct {
	OrganizationID uuid.UUID
	Action         Action
	Actor          AuditActor
	Outcome        Outcome
	Target         AuditTarget
	Reason         string
	Context        AuditContext
}

// NewAuditEvent builds a validated audit event: the action must be catalogued
// (ErrUnknownAction), the organization present (the chain is per tenant), the
// actor identifiable (a subject) and the outcome defined. It mints a UUIDv7
// event id and stamps the schema version. It does NOT assign seq/hash — that is
// the write path's job (T-003/T-004), which wraps this in a SealedEvent.
func NewAuditEvent(in AuditEventInput) (AuditEvent, error) {
	if !in.Action.Valid() {
		return AuditEvent{}, fmt.Errorf("%w: %q", ErrUnknownAction, in.Action)
	}
	if in.OrganizationID == uuid.Nil {
		return AuditEvent{}, fmt.Errorf("%w: organização obrigatória (cadeia por tenant)", ErrInvalidAuditEvent)
	}
	if in.Actor.IdentitySubject == "" {
		return AuditEvent{}, fmt.Errorf("%w: ator sem subject", ErrInvalidAuditEvent)
	}
	switch in.Outcome {
	case Allowed, Denied, Failed:
	default:
		return AuditEvent{}, fmt.Errorf("%w: outcome indefinido", ErrInvalidAuditEvent)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return AuditEvent{}, fmt.Errorf("audit: geração de UUIDv7 falhou: %w", err)
	}
	return AuditEvent{
		SchemaVersion:  AuditSchemaVersion,
		EventID:        id,
		OrganizationID: in.OrganizationID,
		Action:         in.Action,
		Actor:          in.Actor,
		Target:         in.Target,
		Outcome:        in.Outcome,
		Reason:         in.Reason,
		Context:        in.Context,
	}, nil
}

// SealedEvent is a persisted event: the canonical content plus the per-tenant
// chain fields assigned at write time (RFC-0003 §3). Seq is the monotonic,
// gapless per-organization sequence; PrevHash/Hash form the chain. The
// canonicalizer (T-002) hashes only Event, never these fields.
type SealedEvent struct {
	Event    AuditEvent
	Seq      int64
	PrevHash []byte
	Hash     []byte
}
