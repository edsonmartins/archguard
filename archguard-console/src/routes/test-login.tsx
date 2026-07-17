// src/routes/test-login.tsx
//
// Drives the E2E test-login server function from a navigable URL so
// Playwright can authenticate with `page.goto('/test-login?u=admin')`
// instead of the OIDC redirect chain. Gated by ARCHGUARD_E2E_LOGIN=1.
//
// Lab/UAT: also accepts uat-* when UAT_PASS_* env is set on the console service.

import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'
import { testLoginFn } from '@/server/test-login'

const LAB_USERS = [
  'testadmin',
  'testuser',
  'uat-admin',
  'uat-op-a',
  'uat-op-b',
  'uat-viewer',
] as const

type LabUser = (typeof LAB_USERS)[number]

function passwordFor(u: LabUser): string | undefined {
  switch (u) {
    case 'testadmin':
      return (
        process.env.ARCHGUARD_TESTADMIN_PASSWORD ||
        process.env.TESTADMIN_PASSWORD ||
        'ArchGuard2026TestAdmin'
      )
    case 'testuser':
      return (
        process.env.ARCHGUARD_TESTUSER_PASSWORD ||
        process.env.TESTUSER_PASSWORD ||
        'ArchGuard2026TestUser'
      )
    case 'uat-admin':
      return process.env.UAT_PASS_uat_admin
    case 'uat-op-a':
      return process.env.UAT_PASS_uat_op_a
    case 'uat-op-b':
      return process.env.UAT_PASS_uat_op_b
    case 'uat-viewer':
      return process.env.UAT_PASS_uat_viewer
    default:
      return undefined
  }
}

const searchSchema = z.object({
  u: z.enum(LAB_USERS),
})

export const Route = createFileRoute('/test-login')({
  validateSearch: searchSchema,
  beforeLoad: async ({ search }) => {
    const password = passwordFor(search.u)
    if (!password) {
      throw redirect({ to: '/login' })
    }

    const result = await testLoginFn({
      data: { username: search.u, password },
    })
    if (!result.success) throw redirect({ to: '/unauthorized' })
    throw redirect({ to: '/dashboard' })
  },
  component: () => null,
})
