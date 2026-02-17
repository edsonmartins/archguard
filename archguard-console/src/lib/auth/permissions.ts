// src/lib/auth/permissions.ts

export type Permission =
  // Identidades
  | 'persons:read'
  | 'persons:create'
  | 'persons:update'
  | 'persons:delete'
  | 'persons:credentials'
  | 'persons:import'
  // Grupos
  | 'groups:read'
  | 'groups:create'
  | 'groups:update'
  | 'groups:delete'
  | 'groups:members'
  // OAuth2
  | 'oauth2:read'
  | 'oauth2:create'
  | 'oauth2:update'
  | 'oauth2:delete'
  | 'oauth2:secrets'
  // Service Accounts
  | 'service_accounts:read'
  | 'service_accounts:create'
  | 'service_accounts:delete'
  | 'service_accounts:tokens'
  // Vault
  | 'vault:read'
  | 'vault:admin'
  // Auditoria
  | 'audit:read'
  | 'audit:export'
  // Sistema
  | 'settings:read'
  | 'settings:update'
  | 'system:admin'

export const ALL_PERMISSIONS: Permission[] = [
  'persons:read',
  'persons:create',
  'persons:update',
  'persons:delete',
  'persons:credentials',
  'persons:import',
  'groups:read',
  'groups:create',
  'groups:update',
  'groups:delete',
  'groups:members',
  'oauth2:read',
  'oauth2:create',
  'oauth2:update',
  'oauth2:delete',
  'oauth2:secrets',
  'service_accounts:read',
  'service_accounts:create',
  'service_accounts:delete',
  'service_accounts:tokens',
  'vault:read',
  'vault:admin',
  'audit:read',
  'audit:export',
  'settings:read',
  'settings:update',
  'system:admin',
]

const GROUP_PERMISSIONS: Record<string, Permission[]> = {
  idm_admins: ['system:admin'],
  idm_people_admins: [
    'persons:read',
    'persons:create',
    'persons:update',
    'persons:delete',
    'persons:credentials',
    'persons:import',
    'groups:read',
    'groups:create',
    'groups:update',
    'groups:members',
  ],
  idm_oauth2_admins: [
    'oauth2:read',
    'oauth2:create',
    'oauth2:update',
    'oauth2:delete',
    'oauth2:secrets',
  ],
  idm_service_desk: ['persons:read', 'persons:credentials', 'groups:read'],
  idm_people_on_boarding: [
    'persons:read',
    'persons:create',
    'persons:import',
    'groups:read',
    'groups:members',
  ],
  archguard_admins: ['system:admin'],
}

export function derivePermissions(groups: string[]): Permission[] {
  const perms = new Set<Permission>()

  for (const group of groups) {
    const direct = GROUP_PERMISSIONS[group]
    if (direct) {
      direct.forEach((p) => perms.add(p))
    }

    // Tenant admin: {tenant}_admins
    if (group.endsWith('_admins') && !GROUP_PERMISSIONS[group]) {
      ;(
        [
          'persons:read',
          'persons:create',
          'persons:update',
          'persons:credentials',
          'groups:read',
          'groups:members',
          'audit:read',
        ] as Permission[]
      ).forEach((p) => perms.add(p))
    }
  }

  if (perms.has('system:admin')) return ALL_PERMISSIONS
  return Array.from(perms)
}

export function hasPermission(
  userPerms: Permission[],
  required: Permission | Permission[],
): boolean {
  if (userPerms.includes('system:admin')) return true
  const perms = Array.isArray(required) ? required : [required]
  return perms.every((p) => userPerms.includes(p))
}

export function hasAnyPermission(
  userPerms: Permission[],
  required: Permission[],
): boolean {
  if (userPerms.includes('system:admin')) return true
  return required.some((p) => userPerms.includes(p))
}
