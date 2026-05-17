// tests/e2e/recycle-bin.spec.ts
//
// Recycle bin lists deleted entries and offers a restore action. We seed an
// entry by creating then deleting a person and then verify it appears in the
// bin. Restore is exercised end-to-end so the person reappears in /identities.

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'
import { createPersonViaUI, openListRow } from './fixtures/factories'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test.describe('recycle bin', () => {
  test('shows the page with search and an empty-state when nothing matches', async ({
    page,
  }) => {
    await page.goto('/recycle-bin')
    await expect(
      page.getByRole('heading', { name: /^lixeira$/i }),
    ).toBeVisible()
    await expect(page.getByPlaceholder(/buscar na lixeira/i)).toBeVisible()

    // A nonexistent search should land on the empty state.
    await page.getByPlaceholder(/buscar na lixeira/i).fill('nonexistent_999_xyz')
    await expect(page.getByText(/lixeira vazia/i)).toBeVisible({
      timeout: 5_000,
    })
  })

  test('a deleted person shows up in the recycle bin', async ({ page }) => {
    const { username, displayName } = await createPersonViaUI(page, 'e2e_rb')

    // Delete via detail page.
    await openListRow(
      page,
      /buscar por nome, email ou username/i,
      displayName,
    )
    await page.getByRole('button', { name: /^excluir$/i }).click()
    await page.getByLabel(/digite.*para confirmar/i).fill(username)
    await page.getByRole('button', { name: /^confirmar$/i }).click()
    await page.waitForURL(/\/identities$/, { timeout: 15_000 })

    // The deleted entry should now be in the bin.
    await page.goto('/recycle-bin')
    await page.getByPlaceholder(/buscar na lixeira/i).fill(username)
    await expect(page.getByText(username).first()).toBeVisible({
      timeout: 10_000,
    })
    await expect(page.getByText(/^pessoa$/i).first()).toBeVisible()
  })

  test('restore brings a deleted person back to /identities', async ({
    page,
  }) => {
    const { username, displayName } = await createPersonViaUI(page, 'e2e_rb_rest')

    // Delete first
    await openListRow(
      page,
      /buscar por nome, email ou username/i,
      displayName,
    )
    await page.getByRole('button', { name: /^excluir$/i }).click()
    await page.getByLabel(/digite.*para confirmar/i).fill(username)
    await page.getByRole('button', { name: /^confirmar$/i }).click()
    await page.waitForURL(/\/identities$/, { timeout: 15_000 })

    // Restore
    await page.goto('/recycle-bin')
    await page.getByPlaceholder(/buscar na lixeira/i).fill(username)
    await expect(page.getByText(username).first()).toBeVisible({
      timeout: 10_000,
    })
    await page.getByRole('button', { name: /restaurar/i }).first().click()

    // Confirm-by-name dialog
    await page.getByLabel(/digite.*para confirmar/i).fill(username)
    await page.getByRole('button', { name: /^confirmar$/i }).click()

    // The user should reappear in /identities.
    await page.goto('/identities')
    await page
      .getByPlaceholder(/buscar por nome, email ou username/i)
      .fill(username)
    await expect(page.getByText(displayName).first()).toBeVisible({
      timeout: 15_000,
    })
  })
})
