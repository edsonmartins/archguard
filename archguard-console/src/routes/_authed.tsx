// src/routes/_authed.tsx

import { createFileRoute, redirect, Outlet } from '@tanstack/react-router'
import { getSessionFn } from '@/server/auth'
import { AppShell } from '@/components/layout/app-shell'
import type { Permission } from '@/lib/auth/permissions'

export interface AuthContext {
  user: {
    id: string
    name: string
    email: string
    displayName: string
  }
  groups: string[]
  permissions: Permission[]
}

export const Route = createFileRoute('/_authed')({
  beforeLoad: async () => {
    const session = await getSessionFn()

    if (!session || !session.isAuthenticated) {
      throw redirect({ to: '/login' })
    }

    if (!session.isAdmin && !session.permissions.length) {
      throw redirect({ to: '/unauthorized' })
    }

    return {
      auth: {
        user: session.user,
        groups: session.groups,
        permissions: session.permissions,
      } satisfies AuthContext,
    }
  },
  component: AuthedLayout,
})

function AuthedLayout() {
  const { auth } = Route.useRouteContext()
  return (
    <AppShell user={auth.user}>
      <Outlet />
    </AppShell>
  )
}
