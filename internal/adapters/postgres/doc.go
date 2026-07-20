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

// Package postgres is the pgx-based persistence adapter for NEW ArchGuard code
// (ADR-0016): explicit SQL over pgx, behind repository interfaces defined in the
// domain. The legacy XORM path stays only for inherited upstream tables.
//
// Foundations provided here:
//   - Pool: a shared pgx connection pool.
//   - WithTx: run one business operation in exactly one transaction (RFC-0002
//     §5). Never make a remote call inside a DB transaction — use the
//     transactional outbox instead (RFC-0004 §4).
//
// Repositories built on this must take their tenant context in the constructor
// (CLAUDE.md §6): cross-tenant access uses a distinct, explicit, audited type.
package postgres
