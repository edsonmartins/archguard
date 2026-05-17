// tests/e2e/auth.spec.ts

import { test, expect } from '@playwright/test'
import { loginAs, logout } from './fixtures/auth'

test.describe('authentication flow', () => {
  test('redirects unauthenticated user from /dashboard to /login', async ({
    page,
  }) => {
    await page.context().clearCookies()
    await page.goto('/dashboard')
    await page.waitForURL(/\/login/, { timeout: 10_000 })
    await expect(
      page.getByRole('button', { name: /entrar com archguard id/i }),
    ).toBeVisible()
  })

  test('admin user logs in and lands on dashboard', async ({ page }) => {
    await loginAs(page, 'admin')
    await expect(page).toHaveURL(/\/dashboard/)
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
  })

  test('logout clears session and protected routes redirect to /login', async ({
    page,
  }) => {
    await loginAs(page, 'admin')
    await expect(page).toHaveURL(/\/dashboard/)

    await logout(page)
    await page.goto('/dashboard')
    await page.waitForURL(/\/login/, { timeout: 10_000 })
  })

  test('login page shows the entry button when arriving fresh', async ({
    page,
  }) => {
    await page.context().clearCookies()
    await page.goto('/login')
    await expect(
      page.getByRole('button', { name: /entrar com archguard id/i }),
    ).toBeEnabled()
  })
})
