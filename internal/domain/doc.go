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

// Package domain and its subpackages hold the ArchGuard identity domain: pure
// business rules with no dependency on the web framework (Beego) or the ORM
// (XORM). This boundary is INV-3 (ADR-0016) and is enforced by the invariant
// suite (test/invariants) via `make deps-check`: a domain package that imports
// the framework or the ORM breaks the build.
//
// The rule exists to keep the framework a replaceable detail: the domain is
// where multi-tenancy, audit, break-glass, step-up and PDP/vault integration
// live, and none of it may reach for HTTP or SQL. Adapters (internal/adapters)
// implement the ports; handlers (internal/http) translate HTTP↔domain.
package domain
