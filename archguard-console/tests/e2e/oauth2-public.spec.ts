// tests/e2e/oauth2-public.spec.ts
//
// Public/PKCE clients are picked from the wizard step 0. The detail page
// should reflect that and the "Tipo" badge should read "Public (PKCE)".

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'
import { createOAuth2ClientViaUI, openListRow } from './fixtures/factories'

test.beforeEach(async ({ page }) => {
  await loginAs(page, 'admin')
})

test('creates a public/PKCE client and detail page reflects the type', async ({
  page,
}) => {
  const client = await createOAuth2ClientViaUI(page, 'public', 'e2e-pub')

  await openListRow(page, /buscar clientes/i, client.displayName)

  await expect(
    page.getByRole('heading', { name: client.displayName }),
  ).toBeVisible({ timeout: 10_000 })
  await expect(page.getByText(/public.*pkce/i).first()).toBeVisible()
})

test('switches between detail tabs for an OAuth2 client', async ({ page }) => {
  const client = await createOAuth2ClientViaUI(page, 'basic', 'e2e-tabs')

  await openListRow(page, /buscar clientes/i, client.displayName)

  for (const tab of [
    /configuração/i,
    /scope maps/i,
    /acesso/i,
    /integração/i,
    /danger zone/i,
  ]) {
    await page.getByRole('tab', { name: tab }).click()
    await expect(page.getByRole('tab', { name: tab })).toHaveAttribute(
      'data-state',
      'active',
    )
  }
})

test('adds a redirect URL through the config tab', async ({ page }) => {
  const client = await createOAuth2ClientViaUI(page, 'basic', 'e2e-redir')
  await openListRow(page, /buscar clientes/i, client.displayName)

  // Configuração tab is the default.
  const newRedirect = `${client.landing}/auth/cb`
  await page
    .getByPlaceholder(/https:\/\/app\.exemplo\.com\/callback/i)
    .fill(newRedirect)
  await page.getByRole('button', { name: /^adicionar$/i }).click()

  await expect(page.getByText(newRedirect).first()).toBeVisible({
    timeout: 10_000,
  })
})
