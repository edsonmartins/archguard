// tests/e2e/persons-with-groups.spec.ts
//
// Person creation wizard step 2: assigning the new person to one or more
// groups. The wizard creates the person first then attempts to add it to
// each chosen group, so we assert that the resulting person carries the
// expected memberships in the detail view.

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'
import { createGroupViaUI, openListRow, stamp } from './fixtures/factories'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test('creates a person with a chosen group via the wizard', async ({ page }) => {
  // 1) seed a fresh group so the search field always finds at least one
  //    non-builtin entry.
  const group = await createGroupViaUI(page, 'e2e_grp_pwz')

  // 2) drive the wizard
  const s = stamp()
  const username = `pwz_${s}`
  const displayName = `PWZ ${s}`
  const email = `${username}@example.test`

  await page.goto('/identities/create')
  await page.getByLabel(/^username \*/i).fill(username)
  await page.getByLabel(/nome de exibição/i).fill(displayName)
  await page.getByPlaceholder(/email@exemplo\.com/i).fill(email)
  await page.getByPlaceholder(/email@exemplo\.com/i).press('Enter')
  await page.getByRole('button', { name: /próximo/i }).click()

  // Step 2: groups
  await page.getByPlaceholder(/buscar grupos/i).fill(group.name)
  await page.getByText(group.name).first().click()

  await page.getByRole('button', { name: /próximo/i }).click()
  await page.getByRole('button', { name: /criar pessoa/i }).click()
  await page.waitForURL(/\/identities$/, { timeout: 15_000 })

  // 3) detail must show the chosen group as a badge
  await openListRow(page, /buscar por nome, email ou username/i, displayName)
  await expect(
    page.getByRole('heading', { name: displayName }),
  ).toBeVisible({ timeout: 10_000 })
  // The Visão Geral tab includes a "Grupos" card that lists memberships.
  await expect(page.getByText(group.name).first()).toBeVisible()
})
