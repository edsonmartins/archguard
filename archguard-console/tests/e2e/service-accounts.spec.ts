// tests/e2e/service-accounts.spec.ts

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test.describe('service accounts', () => {
  test('lists service accounts on /service-accounts', async ({ page }) => {
    await page.goto('/service-accounts')
    await expect(
      page.getByRole('heading', { name: /^service accounts$/i }),
    ).toBeVisible()
    await expect(page.getByPlaceholder(/buscar service accounts/i)).toBeVisible()
  })

  test('creates a service account and finds it in the list', async ({
    page,
  }) => {
    const stamp = Date.now().toString(36)
    const name = `e2e_sa_${stamp}`
    const displayName = `E2E SA ${stamp}`

    await page.goto('/service-accounts')
    await page.getByRole('link', { name: /novo service account/i }).click()
    await page.waitForURL(/\/service-accounts\/create/)

    await page.getByLabel(/nome \(spn\)/i).fill(name)
    await page.getByLabel(/nome de exibição/i).fill(displayName)

    await page.getByRole('button', { name: /^criar service account$/i }).click()
    await page.waitForURL(/\/service-accounts$/, { timeout: 15_000 })

    await page.getByPlaceholder(/buscar service accounts/i).fill(name)
    await expect(page.getByText(displayName).first()).toBeVisible({
      timeout: 10_000,
    })
  })

  test('opens detail page from the list and shows the tokens tab', async ({
    page,
  }) => {
    const stamp = Date.now().toString(36)
    const name = `e2e_sa_det_${stamp}`
    const displayName = `E2E SA Detail ${stamp}`

    // create
    await page.goto('/service-accounts/create')
    await page.getByLabel(/nome \(spn\)/i).fill(name)
    await page.getByLabel(/nome de exibição/i).fill(displayName)
    await page.getByRole('button', { name: /^criar service account$/i }).click()
    await page.waitForURL(/\/service-accounts$/, { timeout: 15_000 })

    // open detail
    await page.getByPlaceholder(/buscar service accounts/i).fill(name)
    await page.getByText(displayName).first().click()
    await expect(page.getByRole('tab', { name: /tokens/i })).toBeVisible({
      timeout: 10_000,
    })
  })
})
