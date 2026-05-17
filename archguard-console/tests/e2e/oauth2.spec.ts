// tests/e2e/oauth2.spec.ts

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test.describe('oauth2 clients', () => {
  test('lists existing clients on /oauth2', async ({ page }) => {
    await page.goto('/oauth2')
    await expect(
      page.getByRole('heading', { name: /oauth2 \/ sso/i }),
    ).toBeVisible()
  })

  test('creates a basic (confidential) client through the wizard', async ({
    page,
  }) => {
    const stamp = Date.now().toString(36)
    const clientId = `e2e-app-${stamp}`
    const displayName = `E2E App ${stamp}`
    const landing = `https://e2e-${stamp}.example.test`

    await page.goto('/oauth2')
    await page.getByRole('link', { name: /novo client/i }).click()
    await page.waitForURL(/\/oauth2\/create/)

    // Step 0 — type. "basic" is the default; advance.
    await page.getByRole('button', { name: /^próximo$/i }).click()

    // Step 1 — config
    await page.getByLabel(/client id \(slug\)/i).fill(clientId)
    await page.getByLabel(/nome de exibição/i).fill(displayName)
    await page.getByLabel(/landing url/i).fill(landing)
    await page.getByRole('button', { name: /^próximo$/i }).click()

    // Step 2 — scope maps (skip, optional)
    await page.getByRole('button', { name: /^próximo$/i }).click()

    // Step 3 — review + submit
    await page.getByRole('button', { name: /^criar client$/i }).click()
    await page.waitForURL(/\/oauth2$/, { timeout: 15_000 })

    await page.getByPlaceholder(/buscar clientes/i).fill(clientId)
    await expect(page.getByText(displayName).first()).toBeVisible({
      timeout: 10_000,
    })
  })

  test('disables advance on the config step until required fields are valid', async ({
    page,
  }) => {
    await page.goto('/oauth2/create')
    await page.getByRole('button', { name: /^próximo$/i }).click()
    // Now on config step. Without name+displayname+landing, "Próximo" is disabled.
    await expect(page.getByRole('button', { name: /^próximo$/i })).toBeDisabled()
  })
})
