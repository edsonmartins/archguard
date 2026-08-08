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

  it('fails closed when RustGuac omits the session or ticket', () => {
    expect(() => buildRustGuacUrls({ session_id: '' }, 'ticket')).toThrow(
      'sessão sem id',
    )
    expect(() => buildRustGuacUrls({ session_id: 'sid' }, '')).toThrow(
      'ticket vazio',
    )
  })
})
