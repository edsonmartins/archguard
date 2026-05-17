// tests/e2e/fixtures/factories.ts
//
// Helpers that drive the UI to create fixtures used by other specs.
// Returning the entity name keeps follow-up assertions simple and lets
// each test stamp its own unique identifier so parallel runs don't clash.

import type { Page } from '@playwright/test'

export function stamp(): string {
  // Only lowercase letters and digits — the OAuth2 client-id slug only
  // accepts `[a-z0-9-]+`, and the SA SPN gets pickier as Kanidm ages.
  return `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`
}

export interface CreatedPerson {
  username: string
  displayName: string
  email: string
}

export async function createPersonViaUI(
  page: Page,
  prefix = 'e2e',
): Promise<CreatedPerson> {
  const s = stamp()
  const username = `${prefix}_${s}`
  const displayName = `${prefix.toUpperCase()} ${s}`
  const email = `${username}@example.test`

  await page.goto('/identities/create')
  await page.getByLabel(/^username \*/i).fill(username)
  await page.getByLabel(/nome de exibição/i).fill(displayName)
  await page.getByPlaceholder(/email@exemplo\.com/i).fill(email)
  await page.getByPlaceholder(/email@exemplo\.com/i).press('Enter')

  await page.getByRole('button', { name: /próximo/i }).click()
  await page.getByRole('button', { name: /próximo/i }).click()
  await page.getByRole('button', { name: /criar pessoa/i }).click()
  await page.waitForURL(/\/identities$/, { timeout: 30_000 })

  return { username, displayName, email }
}

export interface CreatedGroup {
  name: string
  description: string
}

export async function createGroupViaUI(
  page: Page,
  prefix = 'e2e_grp',
): Promise<CreatedGroup> {
  const s = stamp()
  const name = `${prefix}_${s}`
  const description = `Group ${s}`

  await page.goto('/groups/create')
  await page.getByLabel(/nome do grupo \*/i).fill(name)
  await page.getByLabel(/descrição/i).fill(description)
  await page.getByRole('button', { name: /^criar grupo$/i }).click()
  await page.waitForURL(/\/groups$/, { timeout: 30_000 })

  return { name, description }
}

export interface CreatedServiceAccount {
  name: string
  displayName: string
}

export async function createServiceAccountViaUI(
  page: Page,
  prefix = 'e2e_sa',
): Promise<CreatedServiceAccount> {
  const s = stamp()
  const name = `${prefix}_${s}`
  const displayName = `SA ${s}`

  await page.goto('/service-accounts/create')
  await page.getByLabel(/nome \(spn\)/i).fill(name)
  await page.getByLabel(/nome de exibição/i).fill(displayName)
  await page.getByRole('button', { name: /^criar service account$/i }).click()
  await page.waitForURL(/\/service-accounts$/, { timeout: 30_000 })

  return { name, displayName }
}

export interface CreatedOAuth2Client {
  name: string
  displayName: string
  landing: string
}

export async function createOAuth2ClientViaUI(
  page: Page,
  type: 'basic' | 'public' = 'basic',
  prefix = 'e2e-app',
): Promise<CreatedOAuth2Client> {
  const s = stamp()
  const name = `${prefix}-${s}`
  const displayName = `App ${s}`
  const landing = `https://${prefix}-${s}.example.test`

  await page.goto('/oauth2/create')
  // Step 0 — type
  if (type === 'public') {
    await page.getByRole('button', { name: /public.*pkce/i }).click()
  }
  await page.getByRole('button', { name: /^próximo$/i }).click()

  // Step 1 — config
  await page.getByLabel(/client id \(slug\)/i).fill(name)
  await page.getByLabel(/nome de exibição/i).fill(displayName)
  await page.getByLabel(/landing url/i).fill(landing)
  await page.getByRole('button', { name: /^próximo$/i }).click()

  // Step 2 — scope maps (skip)
  await page.getByRole('button', { name: /^próximo$/i }).click()

  // Step 3 — submit
  await page.getByRole('button', { name: /^criar client$/i }).click()
  await page.waitForURL(/\/oauth2$/, { timeout: 30_000 })

  return { name, displayName, landing }
}

/**
 * Open a row in the list page by clicking on the visible display name.
 * Each list page filters by display name, so this reaches the row reliably.
 */
export async function openListRow(
  page: Page,
  searchPlaceholderRegex: RegExp,
  visibleText: string,
): Promise<void> {
  await page.getByPlaceholder(searchPlaceholderRegex).fill(visibleText)
  await page.getByText(visibleText).first().click()
}
