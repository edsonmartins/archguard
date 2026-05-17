// tests/e2e/fixtures/auth.ts
//
// E2E login helper. Uses the programmatic test-login route instead of the
// OIDC redirect, which is unreliable under Playwright + self-signed Kanidm
// cert. The test-login route is gated by ARCHGUARD_E2E_LOGIN=1 server-side.

import type { Page } from '@playwright/test'
import { TEST_USERS } from './test-data'

type UserKey = keyof typeof TEST_USERS

export async function loginAs(page: Page, userKey: UserKey): Promise<void> {
  const user = TEST_USERS[userKey]
  await page.goto(`/test-login?u=${user.username}`)
  await page.waitForURL(/\/dashboard|\/unauthorized/, { timeout: 30_000 })
}

export async function logout(page: Page): Promise<void> {
  await page.context().clearCookies()
  await page.goto('/login')
}
