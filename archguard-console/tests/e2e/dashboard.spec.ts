// tests/e2e/dashboard.spec.ts

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test.describe('dashboard', () => {
  test('shows the four stats cards', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(
      page.getByRole('heading', { name: /^dashboard$/i }),
    ).toBeVisible()

    for (const card of [/pessoas/i, /^grupos$/i, /oauth2 clients/i, /^vault$/i]) {
      await expect(page.getByText(card).first()).toBeVisible()
    }
  })

  test('renders system health for ArchGuard ID and Vault', async ({
    page,
  }) => {
    await page.goto('/dashboard')
    await expect(page.getByText(/archguard id/i)).toBeVisible({
      timeout: 10_000,
    })
    await expect(page.getByText(/archguard vault/i)).toBeVisible()
  })

  test('quick actions link to creation pages', async ({ page }) => {
    await page.goto('/dashboard')
    // The quick actions card uses Link; just verify the labels exist.
    await expect(page.getByRole('link', { name: /nova pessoa/i })).toBeVisible()
    await expect(page.getByRole('link', { name: /novo grupo/i })).toBeVisible()
    await expect(page.getByRole('link', { name: /novo client/i })).toBeVisible()
  })

  test('clicking "Nova Pessoa" navigates to /identities/create', async ({
    page,
  }) => {
    await page.goto('/dashboard')
    await page.getByRole('link', { name: /nova pessoa/i }).first().click()
    await page.waitForURL(/\/identities\/create/)
  })
})
