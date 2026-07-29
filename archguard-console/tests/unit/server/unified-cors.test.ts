// Guards the unified BFF CORS policy from the 2026-07-28 audit.
// The old helper reflected any Origin with Allow-Credentials: true.

import { describe, expect, it, beforeEach, vi } from 'vitest'

let unifiedCorsHeaders: typeof import('@/server/unified-cors').unifiedCorsHeaders

async function load(allow?: string) {
  vi.resetModules()
  if (allow === undefined) delete process.env.UNIFIED_UI_ORIGIN
  else process.env.UNIFIED_UI_ORIGIN = allow
  ;({ unifiedCorsHeaders } = await import('@/server/unified-cors'))
}

function req(origin?: string): Request {
  return new Request('https://console.archgate.com.br/api/unified/v1/connections', {
    headers: origin ? { Origin: origin } : {},
  })
}

beforeEach(() => {
  delete process.env.UNIFIED_UI_ORIGIN
  delete process.env.CORS_ALLOW_ORIGIN
})

describe('unified BFF CORS', () => {
  it('emits nothing when no allowlist is configured', async () => {
    await load(undefined)
    expect(
      unifiedCorsHeaders(req('https://evil.example'), { methods: 'GET, OPTIONS' }),
    ).toEqual({})
  })

  it('never reflects an origin that is not on the allowlist', async () => {
    await load('https://ui.archgate.com.br')
    expect(
      unifiedCorsHeaders(req('https://evil.example'), { methods: 'GET, OPTIONS' }),
    ).toEqual({})
  })

  it('allows a configured origin with credentials and Vary', async () => {
    await load('https://ui.archgate.com.br')
    const h = unifiedCorsHeaders(req('https://ui.archgate.com.br'), {
      methods: 'GET, OPTIONS',
    })
    expect(h['Access-Control-Allow-Origin']).toBe('https://ui.archgate.com.br')
    expect(h['Access-Control-Allow-Credentials']).toBe('true')
    expect(h['Vary']).toBe('Origin')
  })

  it('supports a comma-separated allowlist and trailing slashes', async () => {
    await load('https://a.example/, https://b.example')
    expect(
      unifiedCorsHeaders(req('https://b.example'), { methods: 'GET, OPTIONS' })[
        'Access-Control-Allow-Origin'
      ],
    ).toBe('https://b.example')
    expect(
      unifiedCorsHeaders(req('https://a.example'), { methods: 'GET, OPTIONS' })[
        'Access-Control-Allow-Origin'
      ],
    ).toBe('https://a.example')
  })

  it('never emits a wildcard origin', async () => {
    await load('*')
    expect(
      unifiedCorsHeaders(req('https://evil.example'), { methods: 'GET, OPTIONS' }),
    ).toEqual({})
  })

  it('emits nothing for same-origin requests (no Origin header)', async () => {
    await load('https://ui.archgate.com.br')
    expect(unifiedCorsHeaders(req(), { methods: 'GET, OPTIONS' })).toEqual({})
  })

  it('does not allow client-asserted identity headers', async () => {
    await load('https://ui.archgate.com.br')
    const h = unifiedCorsHeaders(req('https://ui.archgate.com.br'), {
      methods: 'GET, OPTIONS',
    })
    expect(h['Access-Control-Allow-Headers']).not.toMatch(/X-ArchGate-User/i)
    expect(h['Access-Control-Allow-Headers']).not.toMatch(/X-ArchGate-Tenants/i)
  })
})
