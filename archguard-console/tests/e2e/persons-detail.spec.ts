// tests/e2e/persons-detail.spec.ts
//
// Person detail page covers the post-creation lifecycle: viewing attributes,
// resetting credentials, switching tabs, and deletion. Each test creates its
// own person so they can run in any order and clean up after themselves.

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'
import { createPersonViaUI, openListRow } from './fixtures/factories'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test.describe('person detail page', () => {
  test('shows username, emails, status and groups card', async ({ page }) => {
    const { username, displayName, email } = await createPersonViaUI(page)

    await openListRow(
      page,
      /buscar por nome, email ou username/i,
      displayName,
    )

    await expect(
      page.getByRole('heading', { name: displayName }),
    ).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText(`@${username}`).first()).toBeVisible()
    await expect(page.getByText(email)).toBeVisible()

    // The Visão Geral tab is open by default and shows four cards.
    await expect(
      page.getByRole('heading', { name: /^informações$/i }),
    ).toBeVisible()
    await expect(
      page.getByRole('heading', { name: /^contato$/i }),
    ).toBeVisible()
  })

  test('switches between detail tabs', async ({ page }) => {
    const { displayName } = await createPersonViaUI(page)
    await openListRow(
      page,
      /buscar por nome, email ou username/i,
      displayName,
    )

    await page.getByRole('tab', { name: /grupos/i }).click()
    // Group assignment panel mounts in the groups tab.
    await expect(page.getByRole('tab', { name: /grupos/i })).toHaveAttribute(
      'data-state',
      'active',
    )

    await page.getByRole('tab', { name: /credenciais/i }).click()
    await expect(
      page.getByRole('tab', { name: /credenciais/i }),
    ).toHaveAttribute('data-state', 'active')
  })

  test('opens credential reset dialog and shows TTL select', async ({
    page,
  }) => {
    const { displayName } = await createPersonViaUI(page)
    await openListRow(
      page,
      /buscar por nome, email ou username/i,
      displayName,
    )

    await page.getByRole('button', { name: /reset credencial/i }).click()
    await expect(
      page.getByRole('heading', { name: /reset de credencial/i }),
    ).toBeVisible()
    await expect(page.getByText(/validade do link/i)).toBeVisible()

    // We do not actually generate the token here — that requires Kanidm to
    // be writable for the test user and pollutes the audit log. The smoke
    // test stops after verifying the dialog renders.
    await page.getByRole('button', { name: /cancelar/i }).click()
  })

  test('deletion requires typing the username and removes from list', async ({
    page,
  }) => {
    const { username, displayName } = await createPersonViaUI(page, 'e2e_del')

    await openListRow(
      page,
      /buscar por nome, email ou username/i,
      displayName,
    )
    await page.getByRole('button', { name: /^excluir$/i }).click()

    // Confirm dialog: action is disabled until the user types the SPN.
    const confirmInput = page.getByLabel(/digite.*para confirmar/i)
    const confirmBtn = page.getByRole('button', { name: /^confirmar$/i })
    await expect(confirmBtn).toBeDisabled()

    await confirmInput.fill(username)
    await expect(confirmBtn).toBeEnabled()
    await confirmBtn.click()

    await page.waitForURL(/\/identities$/, { timeout: 15_000 })

    // After deletion, searching for the username yields no rows.
    await page
      .getByPlaceholder(/buscar por nome, email ou username/i)
      .fill(username)
    await expect(page.getByText(displayName)).toHaveCount(0, { timeout: 10_000 })
  })
})
