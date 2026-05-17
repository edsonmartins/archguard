// tests/e2e/persons-bulk.spec.ts
//
// The bulk actions toolbar appears after at least one row is selected. We
// validate the toolbar's affordances render — the actual mutation paths
// (add to group, reset credentials) are covered elsewhere; here we only
// check that the bulk UI surfaces them and dismisses correctly.

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'
import { createPersonViaUI } from './fixtures/factories'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
  // Ensure at least 2 rows so the toolbar makes sense.
  await createPersonViaUI(page, 'e2e_bulk_a')
  await createPersonViaUI(page, 'e2e_bulk_b')
})

test.describe('persons bulk actions', () => {
  test('toolbar appears after selecting rows and shows action set', async ({
    page,
  }) => {
    await page.goto('/identities')
    await expect(page.getByRole('heading', { name: /identidades/i })).toBeVisible()

    // The header has "Selecionar todos"; clicking selects the visible page.
    await page.getByLabel(/selecionar todos/i).check()

    await expect(page.getByText(/\d+ selecionado\(s\)/)).toBeVisible({
      timeout: 5_000,
    })
    await expect(
      page.getByRole('button', { name: /adicionar a grupo/i }),
    ).toBeVisible()
    await expect(
      page.getByRole('button', { name: /remover de grupo/i }),
    ).toBeVisible()
    await expect(
      page.getByRole('button', { name: /reset credenciais/i }),
    ).toBeVisible()
    await expect(
      page.getByRole('button', { name: /exportar csv/i }),
    ).toBeVisible()
  })

  test('opens "Adicionar a Grupo" dialog', async ({ page }) => {
    await page.goto('/identities')
    await page.getByLabel(/selecionar todos/i).check()
    await page.getByRole('button', { name: /adicionar a grupo/i }).click()

    await expect(
      page.getByRole('heading', { name: /adicionar.*grupo/i }),
    ).toBeVisible()
    // Cancel without committing.
    await page.keyboard.press('Escape')
  })

  test('CSV export triggers a download', async ({ page }) => {
    await page.goto('/identities')
    await page.getByLabel(/selecionar todos/i).check()

    const downloadPromise = page.waitForEvent('download')
    await page.getByRole('button', { name: /exportar csv/i }).click()
    const download = await downloadPromise

    expect(download.suggestedFilename()).toMatch(/persons-export-.*\.csv/)
  })

  test('clearing selection hides the toolbar', async ({ page }) => {
    await page.goto('/identities')
    await page.getByLabel(/selecionar todos/i).check()
    await expect(page.getByText(/selecionado\(s\)/)).toBeVisible()

    await page.getByLabel(/selecionar todos/i).uncheck()
    await expect(page.getByText(/selecionado\(s\)/)).toHaveCount(0)
  })
})
