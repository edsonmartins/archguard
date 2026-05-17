// Diagnostic — enumerates all critical/serious axe violations across pages.
// Skipped from the regular suite via test.skip below; flip the flag to run.

import { test } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { loginAs } from './fixtures/auth'

const RUN = process.env.A11Y_DEBUG === '1'

test.describe('a11y debug enumeration', () => {
  test.skip(!RUN, 'Set A11Y_DEBUG=1 to enumerate violations')

  test('list all critical/serious violations', async ({ page }) => {
    test.setTimeout(120_000)
    await loginAs(page, 'admin')

    for (const path of [
      '/dashboard',
      '/identities',
      '/groups',
      '/oauth2',
      '/service-accounts',
      '/audit',
      '/settings',
    ]) {
      await page.goto(path)
      await page
        .getByRole('heading', { level: 1 })
        .first()
        .waitFor({ timeout: 5000 })
        .catch(() => {})
      const r = await new AxeBuilder({ page })
        .withTags(['wcag2a', 'wcag2aa'])
        .disableRules(['color-contrast'])
        .analyze()
      const blockers = r.violations.filter(
        (v) => v.impact === 'critical' || v.impact === 'serious',
      )
      console.log(`\n=== ${path} (${blockers.length} blockers) ===`)
      for (const v of blockers) {
        console.log(
          `  [${v.impact}] ${v.id}: ${v.description} (${v.nodes.length}x)`,
        )
        const sample = v.nodes[0]?.html?.slice(0, 200)
        if (sample) console.log(`    sample: ${sample}`)
      }
    }
  })
})
