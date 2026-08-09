import { test, expect } from '@playwright/test'

// Requires a real ArchGuard session cookie and a lab target. The test is
// skipped in ordinary CI; the dev/staging smoke enables it explicitly.
const cookie = process.env.ARCHGATE_E2E_SESSION_COOKIE
const target = process.env.ARCHGATE_E2E_RUSTGUAC_TARGET || 'archgate-lab-ssh'

test.describe('RustGuac browser session', () => {
  test.skip(!cookie, 'set ARCHGATE_E2E_SESSION_COOKIE for the authenticated lab smoke')

  test('opens a session, reaches the WebSocket endpoint and closes it', async ({ request, page }) => {
    const session = await request.post('/api/unified/v1/sessions', {
      headers: { Cookie: `archguard_session=${cookie}` },
      data: { target, protocol: 'ssh' },
    })
    expect(session.ok()).toBeTruthy()
    const body = await session.json()
    expect(body.session_id).toBeTruthy()
    expect(body.ticket || body.ws_url || body.tunnel_url).toBeTruthy()

    await page.goto('/connect')
    const wsUrl = body.ws_url || body.tunnel_url
    const opened = await page.evaluate(async (url) => {
      const ws = new WebSocket(url)
      await new Promise<void>((resolve, reject) => {
        ws.onopen = () => resolve()
        ws.onerror = () => reject(new Error('WebSocket failed'))
      })
      ws.close()
      return true
    }, wsUrl)
    expect(opened).toBe(true)

    const closed = await request.delete(`/api/unified/v1/sessions/${body.session_id}`, {
      headers: { Cookie: `archguard_session=${cookie}` },
    })
    expect([200, 204]).toContain(closed.status())
  })
})
