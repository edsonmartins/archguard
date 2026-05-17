// tests/e2e/service-accounts-tokens.spec.ts
//
// Service-account token lifecycle (generate, reveal-once, revoke) and
// confirm-by-name deletion of the SA itself.

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'
import { createServiceAccountViaUI, openListRow } from './fixtures/factories'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test.describe('service account tokens', () => {
  test('generates a token, shows it once, then closes', async ({ page }) => {
    const sa = await createServiceAccountViaUI(page, 'e2e_sa_tok')
    await openListRow(page, /buscar service accounts/i, sa.displayName)

    // Tokens tab is active by default; click "Gerar Token".
    await page.getByRole('button', { name: /gerar token/i }).first().click()

    await expect(
      page.getByRole('heading', { name: /gerar token de api/i }),
    ).toBeVisible()
    await page.getByLabel(/label do token/i).fill('e2e-pipeline')
    // Default expiry is fine.
    await page.getByRole('button', { name: /^gerar token$/i }).click()

    // Reveal screen: warns the token can't be shown again, offers Fechar.
    await expect(
      page.getByText(/copie agora.*não será exibido novamente/i),
    ).toBeVisible({ timeout: 15_000 })
    // The token value is rendered into a readonly input.
    const tokenValue = await page
      .getByRole('textbox')
      .last()
      .inputValue()
    expect(tokenValue.length).toBeGreaterThan(20)

    await page.getByRole('button', { name: /^fechar$/i }).click()

    // The token now appears in the list with its label.
    await expect(page.getByText(/e2e-pipeline/).first()).toBeVisible({
      timeout: 10_000,
    })
  })

  test('revokes an existing token via the X button on the row', async ({
    page,
  }) => {
    const sa = await createServiceAccountViaUI(page, 'e2e_sa_rev')
    await openListRow(page, /buscar service accounts/i, sa.displayName)
    await page.getByRole('button', { name: /gerar token/i }).first().click()
    await page.getByLabel(/label do token/i).fill('e2e-revoke-me')
    await page.getByRole('button', { name: /^gerar token$/i }).click()
    await page.getByRole('button', { name: /^fechar$/i }).click()
    await expect(page.getByText(/e2e-revoke-me/)).toBeVisible({
      timeout: 10_000,
    })

    // Locate the row by its label and click the destructive button next to it.
    const tokenRow = page
      .locator('div')
      .filter({ hasText: 'e2e-revoke-me' })
      .first()
    await tokenRow.locator('button').last().click()

    await expect(page.getByText(/e2e-revoke-me/)).toHaveCount(0, {
      timeout: 10_000,
    })
  })
})

test('deletes a service account via danger zone confirm-by-name', async ({
  page,
}) => {
  const sa = await createServiceAccountViaUI(page, 'e2e_sa_del')
  await openListRow(page, /buscar service accounts/i, sa.displayName)

  await page.getByRole('button', { name: /^excluir$/i }).click()
  const confirmBtn = page.getByRole('button', { name: /^confirmar$/i })
  await expect(confirmBtn).toBeDisabled()
  await page.getByLabel(/digite.*para confirmar/i).fill(sa.name)
  await expect(confirmBtn).toBeEnabled()
  await confirmBtn.click()

  await page.waitForURL(/\/service-accounts$/, { timeout: 15_000 })
  await page.getByPlaceholder(/buscar service accounts/i).fill(sa.name)
  await expect(page.getByText(sa.displayName)).toHaveCount(0, {
    timeout: 10_000,
  })
})
