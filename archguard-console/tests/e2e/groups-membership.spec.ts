// tests/e2e/groups-membership.spec.ts
//
// Adds and removes members through the group detail page. Each test creates
// fresh fixtures (one group, one or more persons) so they can run in any
// order against a long-lived Kanidm.

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'
import {
  createGroupViaUI,
  createPersonViaUI,
  openListRow,
} from './fixtures/factories'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test('add a member through the "Adicionar Membros" dialog', async ({
  page,
}) => {
  const group = await createGroupViaUI(page, 'e2e_grp_mem')
  const person = await createPersonViaUI(page, 'e2e_mem')

  // createPersonViaUI ends on /identities; navigate back to the groups list
  // before opening the freshly created group's detail.
  await page.goto('/groups')
  await openListRow(page, /buscar grupos/i, group.name)

  await page.getByRole('button', { name: /adicionar membros/i }).click()
  await expect(
    page.getByRole('heading', { name: /adicionar membros/i }),
  ).toBeVisible()

  await page.getByPlaceholder(/buscar/i).last().fill(person.username)
  await page.getByText(person.displayName).first().click()

  // Confirm the action: the dialog has its own primary button.
  await page.getByRole('button', { name: /^adicionar$/i }).click()

  // After the request lands, the member should show up in the members list.
  await expect(
    page.getByText(`@${person.username}`).first(),
  ).toBeVisible({ timeout: 10_000 })
})

test('remove a member by clicking the X next to their row', async ({
  page,
}) => {
  const group = await createGroupViaUI(page, 'e2e_grp_rm')
  const person = await createPersonViaUI(page, 'e2e_rm')

  // First add the person, then remove via the X.
  await page.goto('/groups')
  await openListRow(page, /buscar grupos/i, group.name)
  await page.getByRole('button', { name: /adicionar membros/i }).click()
  await page.getByPlaceholder(/buscar/i).last().fill(person.username)
  await page.getByText(person.displayName).first().click()
  await page.getByRole('button', { name: /^adicionar$/i }).click()
  await expect(
    page.getByText(`@${person.username}`).first(),
  ).toBeVisible({ timeout: 10_000 })

  // The member row has an icon button without text; locate by adjacency
  // to the username and click the last icon button in that row.
  const memberRow = page
    .locator('div')
    .filter({ hasText: `@${person.username}` })
    .first()
  await memberRow.locator('button').last().click()

  await expect(page.getByText(`@${person.username}`)).toHaveCount(0, {
    timeout: 10_000,
  })
})
