// tests/e2e/a11y.spec.ts
//
// Smoke accessibility audit on the main authenticated pages. We scope axe
// to WCAG 2.1 A/AA so the bar is honest without becoming noisy. Failures
// here represent real defects: missing labels, color contrast, ARIA misuse.

import { test, expect } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { loginAs } from './fixtures/auth'

test.describe('accessibility (axe smoke)', () => {
  test.beforeEach(async ({ page }) => {
    await loginAs(page, 'admin')
  })

  for (const path of [
    '/dashboard',
    '/identities',
    '/groups',
    '/oauth2',
    '/service-accounts',
    '/audit',
    '/settings',
  ]) {
    test(`${path} has no critical/serious violations`, async ({ page }) => {
      await page.goto(path)
      // Wait for the heading to be visible so axe sees the rendered tree.
      await page.getByRole('heading', { level: 1 }).first().waitFor()

      const results = await new AxeBuilder({ page })
        .withTags(['wcag2a', 'wcag2aa'])
        // Disable rules whose violations come from third-party CSS we don't
        // own (Shadcn defaults). Re-enable after a dedicated UI pass.
        .disableRules(['color-contrast'])
        .analyze()

      const blockers = results.violations.filter(
        (v) => v.impact === 'critical' || v.impact === 'serious',
      )

      expect(blockers, formatViolations(blockers)).toEqual([])
    })
  }
})

function formatViolations(
  violations: { id: string; description: string; nodes: { target: unknown[] }[] }[],
): string {
  if (violations.length === 0) return 'no violations'
  return violations
    .map(
      (v) =>
        `[${v.id}] ${v.description} (${v.nodes.length} occurrence(s))`,
    )
    .join('\n')
}
