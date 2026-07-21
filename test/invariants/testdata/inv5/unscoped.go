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

// Package inv5testdata carries INJECTED INV-5 violations for the detector's
// self-test (TestSelfINV5DetectsInjectedViolations). It is NEVER built into
// the product (testdata is ignored by the go tool). Three violations, one per
// guarded table, plus one properly scoped query that must NOT be flagged.
package inv5testdata

const (
	// VIOLATION: SELECT on membership without tenant predicate.
	badSelect = `SELECT id, status FROM membership WHERE status = 'active'`
	// VIOLATION: UPDATE on auth_session without WHERE at all.
	badUpdate = `UPDATE auth_session SET status = 'revoked'`
	// VIOLATION: INSERT into role_assignment without the scope column.
	badInsert = `INSERT INTO role_assignment (id, role_id) VALUES ($1, $2)`
	// OK: scoped query — must not be flagged.
	okSelect = `SELECT id FROM membership WHERE organization_id = $1`
)
