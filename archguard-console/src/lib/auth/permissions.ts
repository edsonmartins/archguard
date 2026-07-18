// src/lib/auth/permissions.ts

import { PLATFORM_ADMIN_GROUPS } from './roles'

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
  // Sites / clientes ArchGate (conectividade + inventário)
  | 'sites:read'
  | 'sites:create'
  | 'sites:update'
  | 'sites:delete'
  // Gateways (Warpgate / Guacamole control plane)
  | 'gateways:read'
  | 'gateways:manage'
  // OpenBao / secrets control plane
  | 'secrets:read'
  | 'secrets:manage'
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
  'sites:read',
  'sites:create',
  'sites:update',
  'sites:delete',
  'gateways:read',
  'gateways:manage',
  'secrets:read',
  'secrets:manage',
  'settings:read',
  'settings:update',
  'system:admin',
]

/** Tenant Admin: manage users/groups of own tenant (5.3). No OAuth2/SA/system. */
const TENANT_ADMIN_PERMS: Permission[] = [
  'persons:read',
  'persons:create',
  'persons:update',
  'persons:credentials',
  'persons:import',
  'groups:read',
  'groups:members',
  'audit:read',
  'sites:read',
  'sites:update',
  'gateways:read',
  'secrets:read',
]

/** Operator / Viewer: read-only authorized scope (5.4). */
const OPERATOR_READ_PERMS: Permission[] = [
  'persons:read',
  'groups:read',
  'sites:read',
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
  archguard_super_admins: ['system:admin'],
  // Global Tenant Admin role (scoped in UI by tenant_* membership)
  archguard_tenant_admins: TENANT_ADMIN_PERMS,
  // Operator (Fase 1 ArchGate)
  archguard_users: OPERATOR_READ_PERMS,
  archguard_viewers: OPERATOR_READ_PERMS,
  archguard_service_desk: [
    'persons:read',
    'persons:credentials',
    'groups:read',
    'sites:read',
  ],
}

// Super admins get gateways via system:admin → ALL_PERMISSIONS.
// Explicit grant for tenant admins already includes gateways:read.

export function derivePermissions(groups: string[]): Permission[] {
  const perms = new Set<Permission>()

  for (const group of groups) {
    const name = group.includes('@') ? group.split('@')[0]! : group
    const direct = GROUP_PERMISSIONS[name]
    if (direct) {
      direct.forEach((p) => perms.add(p))
      continue
    }

    // Legacy tenant admin: {tenant}_admins (not platform)
    if (
      name.endsWith('_admins') &&
      !PLATFORM_ADMIN_GROUPS.has(name) &&
      !name.startsWith('idm_') &&
      !name.startsWith('archguard_')
    ) {
      TENANT_ADMIN_PERMS.forEach((p) => perms.add(p))
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
