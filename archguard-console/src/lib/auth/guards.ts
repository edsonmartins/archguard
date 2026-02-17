// src/lib/auth/guards.ts

import { redirect } from '@tanstack/react-router'
import type { Permission } from './permissions'

interface AuthContext {
  user: {
    id: string
    name: string
    email: string
    displayName: string
  }
  groups: string[]
  permissions: Permission[]
}

export function requirePermission(...perms: Permission[]) {
  return ({ context }: { context: { auth: AuthContext } }) => {
    const has = perms.every(
      (p) =>
        context.auth.permissions.includes(p) ||
        context.auth.permissions.includes('system:admin'),
    )
    if (!has) {
      throw redirect({ to: '/unauthorized' })
    }
  }
}
