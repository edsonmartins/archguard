// tests/e2e/csv-import-full.spec.ts
//
// Drives the CSV import wizard end-to-end: upload → mapping → validate →
// import → completion. The CSV uses unique identifiers so the test is
// idempotent against a long-lived Kanidm.

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'
import { stamp } from './fixtures/factories'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test('CSV import: upload → validate → import → completion', async ({
  page,
}) => {
  const s = stamp()
  const u1 = `csv_${s}_1`
  const u2 = `csv_${s}_2`
  const csv =
    'username,displayname,email\n' +
    `${u1},CSV User One ${s},${u1}@example.test\n` +
    `${u2},CSV User Two ${s},${u2}@example.test\n`

  await page.goto('/identities/import')
  await expect(
    page.getByRole('heading', { name: /importar csv/i }),
  ).toBeVisible()

  await page.setInputFiles('input[type="file"]', {
    name: `e2e-${s}.csv`,
    mimeType: 'text/csv',
    buffer: Buffer.from(csv, 'utf-8'),
  })
  await expect(page.getByText(/2 registros encontrados/i)).toBeVisible({
    timeout: 5_000,
  })
  await page.getByRole('button', { name: /^próximo$/i }).click()

  // Step 1 → 2: validation
  await page.getByRole('button', { name: /^validar$/i }).click()
  await expect(
    page.getByRole('button', { name: /importar 2 registros/i }),
  ).toBeVisible({ timeout: 10_000 })

  // Step 2 → 3: actual import (writes to Kanidm)
  await page.getByRole('button', { name: /importar 2 registros/i }).click()

  // Step 3: completion screen lets the user navigate back to the list.
  await expect(
    page.getByRole('button', { name: /^concluir$/i }),
  ).toBeVisible({ timeout: 30_000 })

  await page.getByRole('button', { name: /^concluir$/i }).click()
  await page.waitForURL(/\/identities$/, { timeout: 10_000 })

  // The first imported user should be visible in the list.
  await page
    .getByPlaceholder(/buscar por nome, email ou username/i)
    .fill(u1)
  await expect(page.getByText(`CSV User One ${s}`).first()).toBeVisible({
    timeout: 10_000,
  })
})
