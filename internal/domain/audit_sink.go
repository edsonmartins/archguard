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
)

// ErrAuditUnavailable is returned when an event could not be recorded durably.
// By I-5.4 a privileged operation whose audit cannot be persisted is DENIED —
// this sentinel lets the caller surface "audit unavailable" as the denial
// reason (RFC-0003 §7, spec scenario "Auditoria indisponível") without ever
// letting the operation proceed unrecorded.
var ErrAuditUnavailable = errors.New("audit: trilha indisponível — operação negada (I-5.4)")

// AuditSink is the synchronous, durable audit write (RFC-0003 §7). It is the
// REAL trail that the provisional 002 seams (AccessAuditor, SessionAuditor)
// pointed at; wiring the callers to it is instrumentation (T-017). Record
// persists the event and returns it sealed (with its chain seq/hash); a failure
// means the event was NOT durably written, and by I-5.4 the caller must deny
// the privileged operation.
//
// For a privileged operation the durable write MUST share the operation's
// database transaction so the two are atomic (the postgres implementation
// exposes an in-transaction AppendTx for that). Record is the standalone form,
// for callers that are not already in a transaction.
type AuditSink interface {
	Record(ctx context.Context, in AuditEventInput) (SealedEvent, error)
}

// RequirePrivileged reports whether an action must go through the SYNCHRONOUS
// fail-closed sink (its assurance level is L3) rather than the asynchronous
// queue (T-009). Non-privileged events may be queued; privileged ones must be
// durable before the operation completes.
func (a Action) RequirePrivileged() bool {
	return a.AssuranceLevel() == L3
}

// RecordOrDeny records the event and, on failure, wraps the error as
// ErrAuditUnavailable so the caller denies with a clear reason (I-5.4). It is
// the small helper the privileged flows use so the fail-closed rule is written
// once, not re-derived at each call site.
func RecordOrDeny(ctx context.Context, sink AuditSink, in AuditEventInput) (SealedEvent, error) {
	sealed, err := sink.Record(ctx, in)
	if err != nil {
		return SealedEvent{}, fmt.Errorf("%w: %v", ErrAuditUnavailable, err)
	}
	return sealed, nil
}
