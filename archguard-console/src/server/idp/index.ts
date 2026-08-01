// Identity-admin selection.
//
// `ARCHGATE_IDP` picks the adapter. It defaults to `kanidm` so an existing
// deployment keeps its behaviour until the ArchGuard cutover is deliberately
// switched on.

import { archguardAdmin } from './archguard'
import { kanidmAdmin } from './kanidm'
import type { IdentityAdmin } from './types'

export type IdpKind = IdentityAdmin['kind']

export function idpKind(): IdpKind {
  return process.env.ARCHGATE_IDP === 'archguard' ? 'archguard' : 'kanidm'
}

export function identityAdmin(): IdentityAdmin {
  return idpKind() === 'archguard' ? archguardAdmin : kanidmAdmin
}

/** Idempotent group creation on the active IdP. */
export function ensureGroup(name: string, description?: string) {
  return identityAdmin().ensureGroup(name, description)
}

/**
 * Tenant groups are stored with the IdP-qualified form in some inventories
 * (`tenant_x@domain`); normalize before creating.
 */
export function ensureTenantGroup(tenantGroup: string, cliente?: string) {
  const name = tenantGroup.includes('@')
    ? tenantGroup.split('@')[0]!
    : tenantGroup
  const description = cliente
    ? `ArchGate tenant for ${cliente} (${name})`
    : `ArchGate tenant ${name}`
  return identityAdmin().ensureGroup(name, description)
}

export function addUserToGroup(username: string, group: string) {
  return identityAdmin().addUserToGroup(username, group)
}

export function disableUser(username: string) {
  return identityAdmin().disableUser(username)
}

export function identityAdminConfigured(): boolean {
  return identityAdmin().configured()
}

export * from './groups'
export type { AdminStep, EnsureGroupResult, IdentityAdmin } from './types'
