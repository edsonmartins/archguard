// src/routes/test-login.tsx
//
// Drives the E2E test-login server function from a navigable URL so
// Playwright can authenticate with `page.goto('/test-login?u=admin')`
// instead of the OIDC redirect chain. Disabled in production.

import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'
import { testLoginFn } from '@/server/test-login'

const TEST_PASSWORDS: Record<string, string> = {
  testadmin: 'ArchGuard2026TestAdmin',
  testuser: 'ArchGuard2026TestUser',
}

const searchSchema = z.object({
  u: z.enum(['testadmin', 'testuser']),
})

export const Route = createFileRoute('/test-login')({
  validateSearch: searchSchema,
  beforeLoad: async ({ search }) => {
    const password = TEST_PASSWORDS[search.u]
    if (!password) throw redirect({ to: '/login' })

    const result = await testLoginFn({
      data: { username: search.u, password },
    })
    if (!result.success) throw redirect({ to: '/unauthorized' })
    throw redirect({ to: '/dashboard' })
  },
  component: () => null,
})
