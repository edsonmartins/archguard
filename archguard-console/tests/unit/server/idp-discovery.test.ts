// Kanidm publishes one issuer per OIDC client; ArchGuard publishes a single
// issuer for the deployment. Everything else (jwks_uri, userinfo_endpoint) is
// read from the document, so the discovery URL is the only shape that changes.

import { describe, expect, it, beforeEach, afterEach, vi } from 'vitest'

let mod: typeof import('@/server/idp/discovery')
const fetchMock = vi.fn()

async function load(idp?: string, base?: string) {
  vi.resetModules()
  if (idp === undefined) delete process.env.ARCHGATE_IDP
  else process.env.ARCHGATE_IDP = idp
  if (base === undefined) delete process.env.ARCHGUARD_ID_URL
  else process.env.ARCHGUARD_ID_URL = base
  mod = await import('@/server/idp/discovery')
}

function doc(extra: Record<string, unknown> = {}) {
  return new Response(
    JSON.stringify({
      issuer: 'https://app.archguard.com.br',
      jwks_uri: 'https://app.archguard.com.br/.well-known/jwks',
      ...extra,
    }),
    { status: 200 },
  )
}

beforeEach(() => {
  fetchMock.mockReset()
  vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('discoveryUrl', () => {
  it('uses the per-client path on Kanidm', async () => {
    await load('kanidm', 'https://id.archgate.com.br')
    expect(mod.discoveryUrl('archgate-connect')).toBe(
      'https://id.archgate.com.br/oauth2/openid/archgate-connect/.well-known/openid-configuration',
    )
  })

  it('uses the deployment root on ArchGuard', async () => {
    await load('archguard', 'https://app.archguard.com.br')
    expect(mod.discoveryUrl('archgate-connect')).toBe(
      'https://app.archguard.com.br/.well-known/openid-configuration',
    )
  })

  it('defaults to Kanidm so an existing deploy is untouched', async () => {
    await load(undefined, 'https://id.archgate.com.br')
    expect(mod.discoveryUrl('archguard-console')).toContain('/oauth2/openid/')
  })

  it('tolerates a trailing slash on the base', async () => {
    await load('archguard', 'https://app.archguard.com.br/')
    expect(mod.discoveryUrl('x')).toBe(
      'https://app.archguard.com.br/.well-known/openid-configuration',
    )
  })

  it('honours an explicit base over the environment', async () => {
    await load('archguard', 'https://app.archguard.com.br')
    expect(mod.discoveryUrl('x', 'https://other.example')).toBe(
      'https://other.example/.well-known/openid-configuration',
    )
  })
})

describe('discover', () => {
  it('returns the document and memoizes it per URL', async () => {
    await load('archguard', 'https://app.archguard.com.br')
    fetchMock.mockResolvedValue(doc({ userinfo_endpoint: 'https://app.archguard.com.br/api/userinfo' }))

    const first = await mod.discover('archgate-connect')
    const second = await mod.discover('archgate-connect')

    expect(first.userinfo_endpoint).toBe('https://app.archguard.com.br/api/userinfo')
    expect(second).toBe(first)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('surfaces the userinfo endpoint instead of a hand-built path', async () => {
    await load('archguard', 'https://app.archguard.com.br')
    fetchMock.mockResolvedValue(
      doc({ userinfo_endpoint: 'https://app.archguard.com.br/api/userinfo' }),
    )
    const d = await mod.discover('archgate-connect')
    // `<issuer>/userinfo` is the Kanidm shape and would 404 on ArchGuard.
    expect(d.userinfo_endpoint).not.toBe(`${d.issuer}/userinfo`)
  })

  it('rejects a document missing issuer or jwks_uri', async () => {
    await load('archguard', 'https://app.archguard.com.br')
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ issuer: 'x' }), { status: 200 }),
    )
    await expect(mod.discover('c')).rejects.toThrow(/jwks_uri/)
  })

  it('does not cache a failure', async () => {
    await load('archguard', 'https://app.archguard.com.br')
    fetchMock
      .mockResolvedValueOnce(new Response('boom', { status: 500 }))
      .mockResolvedValueOnce(doc())

    await expect(mod.discover('c')).rejects.toThrow(/discovery failed/)
    await expect(mod.discover('c')).resolves.toMatchObject({
      issuer: 'https://app.archguard.com.br',
    })
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })
})
