// Identity-admin selection.
//
import { archguardAdmin } from './archguard'
import type { IdentityAdmin } from './types'

export type IdpKind = IdentityAdmin['kind']

export function idpKind(): IdpKind {
  return 'archguard'
}

export function identityAdmin(): IdentityAdmin {
  return archguardAdmin
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
