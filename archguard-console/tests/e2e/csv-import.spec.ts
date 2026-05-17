// tests/e2e/csv-import.spec.ts

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test.describe('CSV import wizard', () => {
  test('parses an uploaded CSV, walks through mapping and validates rows', async ({
    page,
  }) => {
    const stamp = Date.now().toString(36)
    const csv =
      'username,displayname,email\n' +
      `csv_${stamp}_1,User One ${stamp},u1_${stamp}@example.test\n` +
      `csv_${stamp}_2,User Two ${stamp},u2_${stamp}@example.test\n`

    await page.goto('/identities/import')
    await expect(
      page.getByRole('heading', { name: /importar csv/i }),
    ).toBeVisible()

    // Upload via the hidden file input.
    await page.setInputFiles('input[type="file"]', {
      name: `e2e-${stamp}.csv`,
      mimeType: 'text/csv',
      buffer: Buffer.from(csv, 'utf-8'),
    })

    // Step 0 → 1: preview must show "2 registros encontrados".
    await expect(page.getByText(/2 registros encontrados/i)).toBeVisible({
      timeout: 5_000,
    })
    await page.getByRole('button', { name: /^próximo$/i }).click()

    // Step 1 → 2: run validation.
    await page.getByRole('button', { name: /^validar$/i }).click()

    // Step 2 must offer the import button with the count baked into its label.
    await expect(
      page.getByRole('button', { name: /importar 2 registros/i }),
    ).toBeVisible({ timeout: 10_000 })

    // Stop before the actual write: keeps the test idempotent and avoids
    // creating fixtures inside Kanidm.
  })

  test('disables advance when no file is selected', async ({ page }) => {
    await page.goto('/identities/import')
    await expect(
      page.getByRole('button', { name: /^próximo$/i }),
    ).toBeDisabled()
  })
})
