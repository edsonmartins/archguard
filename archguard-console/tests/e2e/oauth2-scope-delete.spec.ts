// tests/e2e/oauth2-scope-delete.spec.ts
//
// Scope maps add/remove and confirm-by-name deletion through the danger zone.

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'
import {
  createGroupViaUI,
  createOAuth2ClientViaUI,
  openListRow,
} from './fixtures/factories'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test('adds a scope map (group → scopes) on the Scope Maps tab', async ({
  page,
}) => {
  // Need a non-builtin group to populate the select on the scope maps tab.
  const group = await createGroupViaUI(page, 'e2e_grp_scp')
  const client = await createOAuth2ClientViaUI(page, 'basic', 'e2e-scp')

  await openListRow(page, /buscar clientes/i, client.displayName)
  await page.getByRole('tab', { name: /scope maps/i }).click()

  await page
    .locator('select')
    .first()
    .selectOption({ label: group.name })
  await page
    .getByPlaceholder(/openid, profile, email/i)
    .fill('openid, profile')
  await page.getByRole('button', { name: /^adicionar$/i }).click()

  // The new scope map row carries the group name and the scopes badges.
  await expect(page.getByText(group.name).first()).toBeVisible({
    timeout: 10_000,
  })
  await expect(page.getByText('openid').first()).toBeVisible()
  await expect(page.getByText('profile').first()).toBeVisible()
})

test('deletes an OAuth2 client through the danger zone', async ({ page }) => {
  const client = await createOAuth2ClientViaUI(page, 'basic', 'e2e-del')

  await openListRow(page, /buscar clientes/i, client.displayName)
  await page.getByRole('tab', { name: /danger zone/i }).click()

  await page.getByRole('button', { name: /^excluir$/i }).click()

  const confirmBtn = page.getByRole('button', { name: /^confirmar$/i })
  await expect(confirmBtn).toBeDisabled()
  await page.getByLabel(/digite.*para confirmar/i).fill(client.name)
  await expect(confirmBtn).toBeEnabled()
  await confirmBtn.click()

  await page.waitForURL(/\/oauth2$/, { timeout: 15_000 })
  await page.getByPlaceholder(/buscar clientes/i).fill(client.name)
  await expect(page.getByText(client.displayName)).toHaveCount(0, {
    timeout: 10_000,
  })
})
