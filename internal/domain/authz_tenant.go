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
	"strings"

	"github.com/google/uuid"
)

// Tenant qualification of authorization refs (T-003 / RFC-0004 §3). Every object
// and subject in the graph is qualified by its organization —
// "org:<orgID>/<type>:<id>" — so a tuple can NEVER relate a subject of one tenant
// to an object of another (INV-5). This is the graph-level face of the two-barrier
// isolation: the resolver stays tolerant of both forms, while these guards, called
// at the PDP boundary (Write, Check), refuse anything that would cross tenants.

// Errors of the tenant-qualification guard.
var (
	// ErrCrossTenantRelation is returned when a tuple or a check relates a subject
	// of one organization to an object of another — a relation crossing tenants,
	// which INV-5 forbids in the authorization graph.
	ErrCrossTenantRelation = errors.New("authz: relação cruzando organizações — rejeitada (INV-5)")
	// ErrUnqualifiedRef is returned when a ref is not tenant-qualified (missing the
	// "org:<id>/" prefix). An unqualified ref cannot be placed in the graph: without
	// an organization it has no tenant to be confined to.
	ErrUnqualifiedRef = errors.New("authz: identificador não qualificado por tenant")
)

// Qualify builds a tenant-qualified ref "org:<orgID>/<type>:<id>". The id is the
// object's own identifier (a UUID string for membership/group/asset_group, or an
// opaque asset identifier); it is not interpreted here.
func Qualify(orgID uuid.UUID, typ ObjectType, id string) string {
	return "org:" + orgID.String() + "/" + string(typ) + ":" + id
}

// QualifyUserset builds a tenant-qualified userset ref
// "org:<orgID>/<type>:<id>#<relation>" (e.g. a group's members).
func QualifyUserset(orgID uuid.UUID, typ ObjectType, id, relation string) string {
	return Qualify(orgID, typ, id) + "#" + relation
}

// RefOrg extracts the organization id from a tenant-qualified ref, tolerating a
// userset suffix. ok is false when the ref carries no "org:<id>/" prefix.
func RefOrg(ref string) (org string, ok bool) {
	if i := strings.IndexByte(ref, '#'); i >= 0 {
		ref = ref[:i]
	}
	const prefix = "org:"
	if !strings.HasPrefix(ref, prefix) {
		return "", false
	}
	rest := ref[len(prefix):]
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 {
		return "", false
	}
	return rest[:slash], true
}

// Qualified reports whether ref carries a tenant qualifier.
func Qualified(ref string) bool {
	_, ok := RefOrg(ref)
	return ok
}

// SameTenant reports whether two refs are both tenant-qualified and belong to the
// same organization. An unqualified ref is never "same tenant" as anything — the
// safe default, so a missing qualifier can never be mistaken for a match.
func SameTenant(a, b string) bool {
	oa, ok1 := RefOrg(a)
	ob, ok2 := RefOrg(b)
	return ok1 && ok2 && oa == ob
}

// ValidateTuple enforces the graph-level tenant isolation on a tuple before it is
// written: both refs MUST be tenant-qualified and MUST name the same organization.
// A missing qualifier is ErrUnqualifiedRef; a mismatch is ErrCrossTenantRelation.
// This is what makes the spec's "Tentativa de relação cruzada" a rejected write.
func ValidateTuple(t RelationTuple) error {
	if !t.Valid() {
		return errors.New("authz: tupla incompleta")
	}
	if !Qualified(t.User) || !Qualified(t.Object) {
		return ErrUnqualifiedRef
	}
	if !SameTenant(t.User, t.Object) {
		return ErrCrossTenantRelation
	}
	return nil
}

// GuardSameTenant refuses a check whose subject and object live in different
// organizations, BEFORE any graph resolution: the spec's "Consulta cruzada" is
// denied outright, never by hoping the graph happens to have no path. Returns
// ErrUnqualifiedRef / ErrCrossTenantRelation, which the PDP maps to a denial.
func GuardSameTenant(user, object string) error {
	if !Qualified(user) || !Qualified(object) {
		return ErrUnqualifiedRef
	}
	if !SameTenant(user, object) {
		return ErrCrossTenantRelation
	}
	return nil
}
