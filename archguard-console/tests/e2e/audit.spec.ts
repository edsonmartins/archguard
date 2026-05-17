// tests/e2e/audit.spec.ts
//
// Activity log surfaces every mutation that crosses the proxy. We seed an
// entry by creating a person, then assert it shows up and the export buttons
// trigger downloads.

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'
import { createPersonViaUI } from './fixtures/factories'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test.describe('audit / activity log', () => {
  test('shows the page and the explanatory banner', async ({ page }) => {
    await page.goto('/audit')
    await expect(
      page.getByRole('heading', { name: /log de atividades/i }),
    ).toBeVisible()
    await expect(
      page.getByText(/kanidm v1\.9 não possui api de auditoria/i),
    ).toBeVisible()
  })

  test('a freshly created person produces a "Criar person" entry', async ({
    page,
  }) => {
    const person = await createPersonViaUI(page, 'e2e_audit')

    await page.goto('/audit')
    await page
      .getByPlaceholder(/buscar por ator, ação ou alvo/i)
      .fill(person.username)

    await expect(page.getByText(/criar person/i).first()).toBeVisible({
      timeout: 10_000,
    })
    await expect(page.getByText(person.username).first()).toBeVisible()
  })

  test('CSV export triggers a download with the expected name', async ({
    page,
  }) => {
    await createPersonViaUI(page, 'e2e_audit_csv') // seed at least one row
    await page.goto('/audit')

    const downloadPromise = page.waitForEvent('download')
    await page.getByRole('button', { name: /^csv$/i }).click()
    const download = await downloadPromise

    expect(download.suggestedFilename()).toMatch(/activity-log-.*\.csv/)
  })

  test('JSON export triggers a download with the expected name', async ({
    page,
  }) => {
    await createPersonViaUI(page, 'e2e_audit_json')
    await page.goto('/audit')

    const downloadPromise = page.waitForEvent('download')
    await page.getByRole('button', { name: /^json$/i }).click()
    const download = await downloadPromise

    expect(download.suggestedFilename()).toMatch(/activity-log-.*\.json/)
  })

  test('shows the recycle bin shortcut on the toolbar', async ({ page }) => {
    await page.goto('/audit')
    await expect(page.getByRole('link', { name: /^lixeira$/i })).toBeVisible()
  })
})
