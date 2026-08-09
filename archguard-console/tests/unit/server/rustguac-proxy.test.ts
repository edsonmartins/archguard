import { describe, expect, it } from 'vitest'
import { buildRustGuacUrls } from '@/server/rustguac-proxy'

describe('RustGuac session contract', () => {
  it('exposes only a single-use ticket URL to the browser', () => {
    const result = buildRustGuacUrls(
      { session_id: 'sid-123' },
      'wst_ticket/with spaces',
      'https://guac.example.test/',
    )

    expect(result.embed_url).toBe(
      'https://guac.example.test/client/sid-123?ticket=wst_ticket%2Fwith%20spaces',
    )
    expect(result.tunnel_url).toBe('wss://guac.example.test/ws/sid-123')
    expect(result.connect_data).toBe('')
    expect(result.expires_in).toBe(30)
    expect(JSON.stringify(result)).not.toContain('Bearer')
  })

  it('preserves absolute RustGuac client URLs without leaking API credentials', () => {
    const result = buildRustGuacUrls(
      {
        session_id: 'sid-456',
        client_url: 'https://guac.example.test/client/sid-456',
        ws_url: 'wss://guac.example.test/ws/sid-456',
      },
      'wst_one-shot',
    )

    expect(result.embed_url).toBe(
      'https://guac.example.test/client/sid-456?ticket=wst_one-shot',
    )
    expect(result.tunnel_url).toBe('wss://guac.example.test/ws/sid-456')
  })

  it('closes a server-side RustGuac session without exposing the API key', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('', { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)
    process.env.RUSTGUAC_ENABLED = '1'
    process.env.RUSTGUAC_URL = 'http://rustguac:8080'
    process.env.RUSTGUAC_API_KEY = 'secret-key'
    const { closeRustGuacSession } = await import('@/server/rustguac-proxy')
    await closeRustGuacSession('session-1')
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('http://rustguac:8080/api/sessions/session-1')
    expect(init.method).toBe('DELETE')
    expect(new Headers(init.headers).get('Authorization')).toBe('Bearer secret-key')
  })

  it('propagates only non-secret session policy controls to RustGuac', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ session_id: 'sid-policy' }), { status: 200 }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ ticket: 'ticket-policy' }), { status: 200 }),
      )
    vi.stubGlobal('fetch', fetchMock)
    process.env.RUSTGUAC_ENABLED = '1'
    process.env.RUSTGUAC_URL = 'http://rustguac:8080'
    process.env.RUSTGUAC_PUBLIC_URL = 'https://guac.example.test'
    process.env.RUSTGUAC_API_KEY = 'secret-key'
    const { issueRustGuacSession } = await import('@/server/rustguac-proxy')

    await issueRustGuacSession({
      protocol: 'ssh',
      hostname: 'lab.internal',
      port: 22,
      username: 'labuser',
      password: 'not-a-browser-secret',
      session_policy: {
        enable_drive: false,
        enable_recording: true,
        disable_copy: true,
        disable_paste: false,
      },
    })

    const body = JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body)) as Record<
      string,
      unknown
    >
    expect(body).toMatchObject({
      session_type: 'ssh',
      hostname: 'lab.internal',
      enable_drive: false,
      enable_recording: true,
      disable_copy: true,
      disable_paste: false,
    })
    expect(JSON.stringify(body)).toContain('not-a-browser-secret')
  })

  it('fails closed when RustGuac omits the session or ticket', () => {
    expect(() => buildRustGuacUrls({ session_id: '' }, 'ticket')).toThrow(
      'sessão sem id',
    )
    expect(() => buildRustGuacUrls({ session_id: 'sid' }, '')).toThrow(
      'ticket vazio',
    )
  })
})
