// src/lib/auth/roles.ts
// Role / tenant derivation for ArchGuard + ArchGate group models.

export type ConsoleRole =
  | 'SUPER_ADMIN'
  | 'TENANT_ADMIN'
  | 'SERVICE_DESK'
  | 'VIEWER'

/** Platform super-admins (full console). */
export const SUPER_ADMIN_GROUPS = [
  'archguard_super_admins',
  'archguard_admins',
  'idm_admins',
] as const

/**
 * Groups that end with `_admins` but are platform-level — NOT tenant admins
 * and not usable as tenant name prefixes.
 */
export const PLATFORM_ADMIN_GROUPS = new Set<string>([
  ...SUPER_ADMIN_GROUPS,
  'archguard_tenant_admins',
  'idm_people_admins',
  'idm_oauth2_admins',
  'idm_recycle_bin_admins',
  'system_admins',
])

/** Strip SPN domain and UUID-looking noise is handled at auth boundary. */
export function stripGroupDomain(group: string): string {
  return group.includes('@') ? group.split('@')[0]! : group
}

export function deriveRole(groups: string[]): ConsoleRole {
  const names = groups.map(stripGroupDomain)

  if (names.some((g) => (SUPER_ADMIN_GROUPS as readonly string[]).includes(g))) {
    return 'SUPER_ADMIN'
  }
  if (
    names.includes('archguard_tenant_admins') ||
    names.some(
      (g) =>
        g.endsWith('_admins') &&
        !PLATFORM_ADMIN_GROUPS.has(g) &&
        !g.startsWith('idm_') &&
        !g.startsWith('archguard_'),
    )
  ) {
    return 'TENANT_ADMIN'
  }
  if (
    names.includes('archguard_service_desk') ||
    names.includes('idm_service_desk')
  ) {
    return 'SERVICE_DESK'
  }
  return 'VIEWER'
}

/**
 * Tenants the principal belongs to.
 *
 * ArchGate convention: membership in `tenant_{slug}` (e.g. tenant_rio_quality).
 * Legacy console convention: `{slug}_admins` → tenant `{slug}`.
 */
export function deriveTenants(groups: string[]): string[] {
  const tenants = new Set<string>()

  for (const raw of groups) {
    const g = stripGroupDomain(raw)

    if (g.startsWith('tenant_') && g !== 'tenant_') {
      tenants.add(g)
      continue
    }

    if (
      g.endsWith('_admins') &&
      !PLATFORM_ADMIN_GROUPS.has(g) &&
      !g.startsWith('idm_') &&
      !g.startsWith('archguard_')
    ) {
      tenants.add(g.replace(/_admins$/, ''))
    }
  }

  return Array.from(tenants).sort()
}

export function isTenantAdminRole(groups: string[]): boolean {
  return deriveRole(groups) === 'TENANT_ADMIN'
}
