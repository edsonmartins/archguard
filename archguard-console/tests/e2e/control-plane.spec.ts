// tests/e2e/control-plane.spec.ts
//
// The ArchGate control-plane screens — the ones a UAT reviewer actually walks
// through — had no E2E coverage at all before the 2026-07-28 audit (P1-4); the
// suite only exercised the identity screens.
//
// These tests deliberately assert on structure (route renders, h1, primary
// controls, tenant scoping) rather than on live upstream data, so they stay
// meaningful whether or not Warpgate/Guacamole/OpenBao are reachable from CI.

import { test, expect } from '@playwright/test'
import { loginAs } from './fixtures/auth'

const PAGES = [
  { path: '/sites', heading: /clientes \/ sites/i },
  { path: '/gateways', heading: /^gateways$/i },
  { path: '/platform', heading: /^plataforma$/i },
  { path: '/secrets', heading: /segredos \(openbao\)/i },
  { path: '/org-accounts', heading: /contas da organização/i },
] as const

test.describe('control plane — admin', () => {
  test.beforeEach(async ({ page }) => {
    await loginAs(page, 'admin')
  })

  for (const { path, heading } of PAGES) {
    test(`${path} renders its page header`, async ({ page }) => {
      await page.goto(path)
      await expect(
        page.getByRole('heading', { level: 1, name: heading }),
      ).toBeVisible({ timeout: 15_000 })
      // A crashed render would leave the shell without its heading; make sure
      // we did not merely land on /unauthorized or /login.
      await expect(page).toHaveURL(new RegExp(path))
    })
  }

  test('/gateways exposes the Warpgate, Guacamole and Sessões tabs', async ({
    page,
  }) => {
    await page.goto('/gateways')
    await expect(page.getByRole('tab', { name: /warpgate/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /guacamole/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /sess/i })).toBeVisible()
  })

  test('/gateways switches to the Guacamole tab', async ({ page }) => {
    await page.goto('/gateways')
    await page.getByRole('tab', { name: /guacamole/i }).click()
    await expect(page.getByRole('tab', { name: /guacamole/i })).toHaveAttribute(
      'aria-selected',
      'true',
    )
  })

  test('/sites offers the search box and the create action', async ({
    page,
  }) => {
    await page.goto('/sites')
    await expect(page.getByPlaceholder(/buscar/i).first()).toBeVisible()
    await expect(
      page.getByRole('link', { name: /novo cliente/i }).first(),
    ).toBeVisible()
  })

  test('/org-accounts lists the inventory without leaking secret material', async ({
    page,
  }) => {
    await page.goto('/org-accounts')
    await expect(
      page.getByRole('heading', { level: 1, name: /contas da organização/i }),
    ).toBeVisible()

    // ADR-013: the list carries references, never credentials.
    const body = (await page.locator('body').innerText()).toLowerCase()
    expect(body).not.toMatch(/password_value|secret_value/)
  })

  test('/secrets never renders a root token or unseal key', async ({ page }) => {
    await page.goto('/secrets')
    await expect(
      page.getByRole('heading', { level: 1, name: /segredos/i }),
    ).toBeVisible({ timeout: 15_000 })

    // Tokens are server-side only (CONSTITUTION §4.5).
    const body = (await page.locator('body').innerText()).toLowerCase()
    expect(body).not.toMatch(/hvs\.[a-z0-9]/i)
    expect(body).not.toMatch(/unseal[_ ]key\s*[:=]\s*\S/i)
  })

  test('/platform reports stack health', async ({ page }) => {
    await page.goto('/platform')
    await expect(
      page.getByRole('heading', { level: 1, name: /plataforma/i }),
    ).toBeVisible({ timeout: 15_000 })
    await expect(page.getByText(/kanidm/i).first()).toBeVisible({
      timeout: 15_000,
    })
  })
})

test.describe('control plane — non-admin (deny by default)', () => {
  test.beforeEach(async ({ page }) => {
    await loginAs(page, 'user')
  })

  test('does not offer the create action on /sites', async ({ page }) => {
    await page.goto('/sites')
    await expect(
      page.getByRole('link', { name: /novo cliente/i }),
    ).toHaveCount(0)
  })

  test('cannot reach the org accounts inventory', async ({ page }) => {
    await page.goto('/org-accounts')
    // Either the route redirects, or it renders without the mutating action.
    const denied = /\/unauthorized|\/login|\/dashboard/.test(page.url())
    if (!denied) {
      await expect(
        page.getByRole('button', { name: /nova conta/i }),
      ).toHaveCount(0)
    }
  })
})
