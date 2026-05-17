// tests/e2e/persons-crud.spec.ts

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'

// Make every test independent: fresh login each time so a failure does not
// leak state into the next case.
test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test.describe('persons CRUD', () => {
  test('lists existing persons on /identities', async ({ page }) => {
    await page.goto('/identities')
    await expect(
      page.getByRole('heading', { name: /identidades/i }),
    ).toBeVisible()
    await expect(
      page.getByPlaceholder(/buscar por nome, email ou username/i),
    ).toBeVisible()
  })

  test('creates a person via the wizard and finds it in the list', async ({
    page,
  }) => {
    const stamp = Date.now().toString(36)
    const username = `e2e_${stamp}`
    const displayName = `E2E ${stamp}`
    const email = `${username}@example.test`

    await page.goto('/identities')
    await page.getByRole('link', { name: /nova pessoa/i }).click()
    await page.waitForURL(/\/identities\/create/)

    // Step 1: basic data
    await page.getByLabel(/^username \*/i).fill(username)
    await page.getByLabel(/nome de exibição/i).fill(displayName)
    await page.getByPlaceholder(/email@exemplo\.com/i).fill(email)
    await page.getByPlaceholder(/email@exemplo\.com/i).press('Enter')

    await page.getByRole('button', { name: /próximo/i }).click()
    // Step 2: groups (skip — leave default).
    await page.getByRole('button', { name: /próximo/i }).click()
    // Step 3: review + submit
    await page.getByRole('button', { name: /criar pessoa/i }).click()

    // Wizard returns to /identities on success
    await page.waitForURL(/\/identities$/, { timeout: 15_000 })

    // The new user shows up in the list (search by username)
    await page
      .getByPlaceholder(/buscar por nome, email ou username/i)
      .fill(username)
    await expect(page.getByText(displayName).first()).toBeVisible({
      timeout: 10_000,
    })
  })

  test('shows validation feedback when the wizard is incomplete', async ({
    page,
  }) => {
    await page.goto('/identities/create')
    // "Próximo" should be disabled until required fields are filled.
    await expect(page.getByRole('button', { name: /próximo/i })).toBeDisabled()
  })
})
