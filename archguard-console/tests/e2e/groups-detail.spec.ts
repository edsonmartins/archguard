// tests/e2e/groups-detail.spec.ts
//
// Group detail page covers viewing, switching tabs, and deletion. Membership
// is exercised in groups-membership.spec.ts.

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'
import { createGroupViaUI, openListRow } from './fixtures/factories'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test.describe('group detail page', () => {
  test('shows the group name, member count and switches tabs', async ({
    page,
  }) => {
    const group = await createGroupViaUI(page, 'e2e_grp_det')
    await openListRow(page, /buscar grupos/i, group.name)

    await expect(
      page.getByRole('heading', { name: group.name }),
    ).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText(group.description)).toBeVisible()

    // Members tab is open by default; with 0 members we see the empty state.
    await expect(page.getByText(/não possui membros/i)).toBeVisible()

    // Config tab exposes the canonical info card.
    await page.getByRole('tab', { name: /configuração/i }).click()
    await expect(
      page.getByRole('heading', { name: /informações do grupo/i }),
    ).toBeVisible()
  })

  test('non-builtin groups expose an "Excluir" button', async ({ page }) => {
    const group = await createGroupViaUI(page, 'e2e_grp_btn')
    await openListRow(page, /buscar grupos/i, group.name)
    await expect(
      page.getByRole('button', { name: /^excluir$/i }),
    ).toBeVisible()
  })

  test('deletes a group with confirm-by-name dialog', async ({ page }) => {
    const group = await createGroupViaUI(page, 'e2e_grp_del')
    await openListRow(page, /buscar grupos/i, group.name)

    await page.getByRole('button', { name: /^excluir$/i }).click()
    const confirmBtn = page.getByRole('button', { name: /^confirmar$/i })
    await expect(confirmBtn).toBeDisabled()

    await page.getByLabel(/digite.*para confirmar/i).fill(group.name)
    await expect(confirmBtn).toBeEnabled()
    await confirmBtn.click()

    await page.waitForURL(/\/groups$/, { timeout: 15_000 })
    await page.getByPlaceholder(/buscar grupos/i).fill(group.name)
    await expect(page.getByText(group.name)).toHaveCount(0, {
      timeout: 10_000,
    })
  })
})
