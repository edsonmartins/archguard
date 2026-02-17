// src/lib/auth/roles.ts

export type ConsoleRole = 'SUPER_ADMIN' | 'TENANT_ADMIN' | 'SERVICE_DESK' | 'VIEWER'

const SUPER_ADMIN_GROUPS = ['archguard_admins', 'idm_admins']

export function deriveRole(groups: string[]): ConsoleRole {
  if (groups.some((g) => SUPER_ADMIN_GROUPS.includes(g))) {
    return 'SUPER_ADMIN'
  }
  if (groups.some((g) => g.endsWith('_admins'))) {
    return 'TENANT_ADMIN'
  }
  if (groups.includes('idm_service_desk')) {
    return 'SERVICE_DESK'
  }
  return 'VIEWER'
}

export function deriveTenants(groups: string[]): string[] {
  return groups
    .filter((g) => g.endsWith('_admins'))
    .filter((g) => !SUPER_ADMIN_GROUPS.includes(g))
    .map((g) => g.replace(/_admins$/, ''))
}
