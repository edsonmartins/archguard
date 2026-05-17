// tests/e2e/settings.spec.ts

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test.describe('settings', () => {
  test('lands on the General tab and shows domain info', async ({ page }) => {
    await page.goto('/settings')
    await expect(
      page.getByRole('heading', { name: /configurações/i }),
    ).toBeVisible()

    await expect(
      page.getByRole('heading', { name: /informações do domínio/i }),
    ).toBeVisible()
    for (const f of [/^domínio$/i, /display name/i, /^status$/i, /versão kanidm/i]) {
      await expect(page.getByText(f).first()).toBeVisible()
    }
  })

  test('Security tab shows credential policy and admin escape hatch', async ({
    page,
  }) => {
    await page.goto('/settings')
    await page.getByRole('tab', { name: /^segurança$/i }).click()

    await expect(
      page.getByRole('heading', { name: /política de credenciais/i }),
    ).toBeVisible()
    for (const f of [
      /credencial mínima/i,
      /expiração de sessão/i,
      /expiração de privilégio/i,
    ]) {
      await expect(page.getByText(f)).toBeVisible()
    }

    // Admin block links to upstream docs.
    await expect(
      page.getByRole('heading', { name: /acesso administrativo/i }),
    ).toBeVisible()
    await expect(
      page.getByRole('link', { name: /documentação/i }),
    ).toHaveAttribute('href', /kanidm\.github\.io/)
  })

  test('System tab shows console + Kanidm version cards', async ({ page }) => {
    await page.goto('/settings')
    await page.getByRole('tab', { name: /^sistema$/i }).click()

    await expect(
      page.getByRole('heading', { name: /informações do console/i }),
    ).toBeVisible()
    await expect(page.getByText(/v1\.0\.0/)).toBeVisible()
    await expect(
      page.getByRole('heading', { name: /backup e manutenção/i }),
    ).toBeVisible()
  })
})
