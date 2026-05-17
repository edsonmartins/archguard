// tests/e2e/vault.spec.ts
//
// Vault dashboard surfaces health (online/offline) and a few stats. The page
// gracefully reports "Offline" when the AliasVault upstream is unreachable,
// so this test is robust to the vault container being down — we only check
// that the page renders the expected widgets.

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test.describe('vault dashboard', () => {
  test('renders status banner and the four stat cards', async ({ page }) => {
    await page.goto('/vault')
    await expect(
      page.getByRole('heading', { name: /archguard vault/i }),
    ).toBeVisible()
    // Status banner shows either Online or Offline.
    await expect(
      page.getByText(/vault (online|offline)/i).first(),
    ).toBeVisible()

    for (const c of [/^vaults$/i, /aliases ativos/i, /^senhas$/i]) {
      await expect(page.getByText(c).first()).toBeVisible()
    }
  })

  test('"Atualizar" refreshes the status query', async ({ page }) => {
    await page.goto('/vault')
    await page.getByRole('button', { name: /atualizar/i }).click()
    // Page stays on /vault and the heading remains visible after the refresh.
    await expect(page).toHaveURL(/\/vault$/)
    await expect(
      page.getByRole('heading', { name: /archguard vault/i }),
    ).toBeVisible()
  })

  test('shows the SMTP status section', async ({ page }) => {
    await page.goto('/vault')
    await expect(
      page.getByRole('heading', { name: /status smtp/i }),
    ).toBeVisible()
    for (const row of [
      /servidor smtp/i,
      /mx configurado/i,
      /spf válido/i,
      /dkim válido/i,
    ]) {
      await expect(page.getByText(row)).toBeVisible()
    }
  })
})
