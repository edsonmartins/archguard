// tests/e2e/permissions.spec.ts
//
// Role-based UI gating. testuser is in archguard_users only and has no
// granular permissions — destructive/creation buttons must be hidden.

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'

test.describe('RBAC — testuser (archguard_users, no granular grants)', () => {
  test.beforeEach(async ({ page }) => {
    await loginAs(page, 'user')
  })

  test('lands authenticated and reaches /dashboard', async ({ page }) => {
    await expect(page).toHaveURL(/\/dashboard|\/identities|\/groups/)
  })

  test('does not see the "Nova Pessoa" button on /identities', async ({
    page,
  }) => {
    await page.goto('/identities')
    await expect(page.getByRole('heading', { name: /identidades/i })).toBeVisible()
    await expect(page.getByRole('link', { name: /nova pessoa/i })).toHaveCount(0)
    await expect(page.getByRole('link', { name: /importar csv/i })).toHaveCount(0)
  })

  test('does not see the "Novo Grupo" button on /groups', async ({ page }) => {
    await page.goto('/groups')
    await expect(page.getByRole('heading', { name: /^grupos$/i })).toBeVisible()
    await expect(page.getByRole('link', { name: /novo grupo/i })).toHaveCount(0)
  })

  test('does not see the "Novo Client" button on /oauth2', async ({ page }) => {
    await page.goto('/oauth2')
    await expect(
      page.getByRole('heading', { name: /oauth2 \/ sso/i }),
    ).toBeVisible()
    await expect(page.getByRole('link', { name: /novo client/i })).toHaveCount(0)
  })

  test('does not see the "Novo Service Account" button', async ({ page }) => {
    await page.goto('/service-accounts')
    await expect(
      page.getByRole('heading', { name: /^service accounts$/i }),
    ).toBeVisible()
    await expect(
      page.getByRole('link', { name: /novo service account/i }),
    ).toHaveCount(0)
  })
})

test.describe('RBAC — testadmin (archguard_admins, all permissions)', () => {
  test.beforeEach(async ({ page }) => {
    await loginAs(page, 'admin')
  })

  test('sees the "Nova Pessoa" CTA', async ({ page }) => {
    await page.goto('/identities')
    await expect(page.getByRole('link', { name: /nova pessoa/i })).toBeVisible()
  })

  test('sees the "Novo Grupo" CTA', async ({ page }) => {
    await page.goto('/groups')
    await expect(page.getByRole('link', { name: /novo grupo/i })).toBeVisible()
  })
})
