// Shared session + permission guards for server functions (control plane).
// Deny-by-default for tenant isolation; never treat empty tenant set as "all sites".

import { getCookie } from '@tanstack/react-start/server'
import { decryptSession } from './session'
import type { SessionData } from './auth'
import {
  derivePermissions,
  type Permission,
} from '@/lib/auth/permissions'
import { deriveTenants, stripGroupDomain } from '@/lib/auth/roles'
import type { Site } from '@/lib/api/types/site'

export function getSessionOrNull(): SessionData | null {
  try {
    const cookie = getCookie('archguard_session')
    if (!cookie) return null
    const s = decryptSession<SessionData>(cookie)
    if (!s?.isAuthenticated) return null
    return s
  } catch {
    return null
  }
}

/** Require an authenticated session cookie. */
export function requireSession(): SessionData {
  const s = getSessionOrNull()
  if (!s) throw new Error('Unauthorized: session required')
  return s
}

export function sessionPermissions(s: SessionData): Permission[] {
  return s.permissions?.length
    ? s.permissions
    : derivePermissions(s.groups)
}

export function hasAnyPerm(s: SessionData, required: Permission[]): boolean {
  const perms = sessionPermissions(s)
  if (perms.includes('system:admin')) return true
  return required.some((p) => perms.includes(p))
}

export function hasAllPerms(s: SessionData, required: Permission[]): boolean {
  const perms = sessionPermissions(s)
  if (perms.includes('system:admin')) return true
  return required.every((p) => perms.includes(p))
}

/**
 * Require at least one of the listed permissions (system:admin always passes).
 * Returns the session for chaining.
 */
export function requireAnyPerm(
  s: SessionData,
  required: Permission[],
  label?: string,
): SessionData {
  if (!hasAnyPerm(s, required)) {
    throw new Error(
      `Forbidden: ${label || required.join(' | ')}`,
    )
  }
  return s
}

/** Require every listed permission (system:admin always passes). */
export function requireAllPerms(
  s: SessionData,
  required: Permission[],
  label?: string,
): SessionData {
  if (!hasAllPerms(s, required)) {
    throw new Error(
      `Forbidden: ${label || required.join(' + ')}`,
    )
  }
  return s
}

/** Actor string for activity log / audit fields. */
export function sessionActor(s: SessionData): string {
  return s.user?.email || s.user?.name || 'console'
}

/**
 * Tenant isolation: system:admin sees all; everyone else is filtered by
 * `deriveTenants`. Empty tenant membership → empty result (deny-by-default).
 */
export function filterSitesByTenant(sites: Site[], s: SessionData): Site[] {
  if (hasAnyPerm(s, ['system:admin'])) return sites
  const tenants = new Set(deriveTenants(s.groups).map(stripGroupDomain))
  if (tenants.size === 0) return []
  return sites.filter((site) =>
    tenants.has(stripGroupDomain(site.tenant_group)),
  )
}

/** Throws if the session cannot access the given site. */
export function assertSiteTenantAccess(site: Site, s: SessionData): void {
  if (filterSitesByTenant([site], s).length === 0) {
    throw new Error('Forbidden: site fora do tenant')
  }
}
