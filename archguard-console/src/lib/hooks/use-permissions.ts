// src/lib/hooks/use-permissions.ts

import { useRouteContext } from '@tanstack/react-router'
import type { Permission } from '../auth/permissions'
import { hasPermission, hasAnyPermission } from '../auth/permissions'
import {
  deriveRole,
  deriveTenants,
  isTenantAdminRole,
} from '../auth/roles'

export function usePermissions() {
  const { auth } = useRouteContext({ from: '/_authed' })

  return {
    groups: auth.groups,
    permissions: auth.permissions,
    role: deriveRole(auth.groups),
    can: (perm: Permission | Permission[]) =>
      hasPermission(auth.permissions, perm),
    canAny: (perms: Permission[]) =>
      hasAnyPermission(auth.permissions, perms),
    isSystemAdmin: auth.permissions.includes('system:admin'),
    isTenantAdmin: isTenantAdminRole(auth.groups),
    tenants: deriveTenants(auth.groups),
  }
}
