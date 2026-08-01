// ArchGuard (Casdoor) identity-admin adapter.
//
// The traps this pins down:
//  - objects are addressed as <org>/<name>
//  - membership lives on the USER, so a bind is read-modify-write
//  - HTTP 200 with {"status":"error"} is a failure
//  - machine access is a short-lived client_credentials bearer, renewed on 401

import { describe, expect, it, beforeEach, afterEach, vi } from 'vitest'

vi.mock('@/server/logger', () => ({
  logger: { warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() },
}))

let archguardAdmin: typeof import('@/server/idp/archguard').archguardAdmin
const fetchMock = vi.fn()

async function load(env: Record<string, string | undefined> = {}) {
  vi.resetModules()
  const defaults: Record<string, string | undefined> = {
    ARCHGUARD_ID_URL: 'https://app.archguard.com.br',
    ARCHGUARD_ORG: 'archgate',
    ARCHGUARD_SA_CLIENT_ID: 'sa-client',
    ARCHGUARD_SA_CLIENT_SECRET: 'sa-secret',
  }
  for (const [k, v] of Object.entries({ ...defaults, ...env })) {
    if (v === undefined) delete process.env[k]
    else process.env[k] = v
  }
  ;({ archguardAdmin } = await import('@/server/idp/archguard'))
}

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status })
}

/** Every call starts with the client_credentials grant. */
function tokenResponse(): Response {
  return json({ access_token: 'bearer-abc', expires_in: 3600 })
}

function urlsCalled(): string[] {
  return fetchMock.mock.calls.map((c) => String(c[0]))
}

beforeEach(() => {
  fetchMock.mockReset()
  vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('configuration', () => {
  it('is unconfigured without client credentials', async () => {
    await load({ ARCHGUARD_SA_CLIENT_SECRET: undefined })
    expect(archguardAdmin.configured()).toBe(false)
  })

  it('is configured with url + client id + secret', async () => {
    await load()
    expect(archguardAdmin.configured()).toBe(true)
  })

  it('skips instead of throwing when unconfigured', async () => {
    await load({ ARCHGUARD_SA_CLIENT_ID: undefined })
    const r = await archguardAdmin.ensureGroup('tenant_x')
    expect(r.action).toBe('skipped')
    expect(fetchMock).not.toHaveBeenCalled()
  })
})

describe('ensureGroup', () => {
  it('reports exists when the group is already there', async () => {
    await load()
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(json({ status: 'ok', data: { name: 'tenant_x' } }))

    const r = await archguardAdmin.ensureGroup('tenant_x')
    expect(r.action).toBe('exists')
    expect(urlsCalled()[1]).toContain('/api/get-group?id=archgate%2Ftenant_x')
  })

  it('creates the group qualified by organization', async () => {
    await load()
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(json({ status: 'error', msg: 'not found' }))
      .mockResolvedValueOnce(json({ status: 'ok' }))

    const r = await archguardAdmin.ensureGroup('tenant_x', 'Cliente X')
    expect(r.action).toBe('created')

    const body = JSON.parse(String(fetchMock.mock.calls[2]![1].body))
    expect(body.owner).toBe('archgate')
    // The name stays flat: Casdoor composes `<owner>/<name>` itself.
    expect(body.name).toBe('tenant_x')
    expect(body.displayName).toBe('Cliente X')
  })

  it('treats a lost creation race as exists', async () => {
    await load()
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(json({ status: 'error' }))
      .mockResolvedValueOnce(json({ status: 'error', msg: 'group already exists' }))

    expect((await archguardAdmin.ensureGroup('tenant_x')).action).toBe('exists')
  })

  it('fails on HTTP 200 with status error', async () => {
    await load()
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(json({ status: 'error' }))
      .mockResolvedValueOnce(json({ status: 'error', msg: 'permission denied' }))

    const r = await archguardAdmin.ensureGroup('tenant_x')
    expect(r.action).toBe('error')
    expect(r.error).toContain('permission denied')
  })
})

describe('addUserToGroup', () => {
  const user = { owner: 'archgate', name: 'op.a', groups: ['archgate/archguard_users'] }

  it('appends the qualified group and writes only the groups column', async () => {
    await load()
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(json({ status: 'ok', data: { name: 'tenant_x' } })) // ensureGroup
      .mockResolvedValueOnce(json({ status: 'ok', data: user })) // get-user
      .mockResolvedValueOnce(json({ status: 'ok' })) // update-user

    const r = await archguardAdmin.addUserToGroup('op.a', 'tenant_x')
    expect(r.ok).toBe(true)

    const [url, init] = fetchMock.mock.calls[3]!
    expect(String(url)).toContain('/api/update-user?id=archgate%2Fop.a')
    expect(String(url)).toContain('columns=groups')
    expect(JSON.parse(String(init.body)).groups).toEqual([
      'archgate/archguard_users',
      'archgate/tenant_x',
    ])
  })

  it('is idempotent when already a member', async () => {
    await load()
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(json({ status: 'ok', data: { name: 'tenant_x' } }))
      .mockResolvedValueOnce(
        json({ status: 'ok', data: { ...user, groups: ['archgate/tenant_x'] } }),
      )

    const r = await archguardAdmin.addUserToGroup('op.a', 'tenant_x')
    expect(r.ok).toBe(true)
    expect(r.detail).toMatch(/already/)
    // No write was issued.
    expect(urlsCalled().some((u) => u.includes('update-user'))).toBe(false)
  })

  it('fails clearly when the user does not exist', async () => {
    await load()
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(json({ status: 'ok', data: { name: 'tenant_x' } }))
      .mockResolvedValueOnce(json({ status: 'error', msg: 'not found' }))

    const r = await archguardAdmin.addUserToGroup('ghost', 'tenant_x')
    expect(r.ok).toBe(false)
    expect(r.detail).toContain('ghost')
  })
})

describe('disableUser', () => {
  it('sets isForbidden without deleting the subject', async () => {
    await load()
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(
        json({ status: 'ok', data: { owner: 'archgate', name: 'op.a' } }),
      )
      .mockResolvedValueOnce(json({ status: 'ok' }))

    const r = await archguardAdmin.disableUser('op.a')
    expect(r.ok).toBe(true)

    const [url, init] = fetchMock.mock.calls[2]!
    expect(String(url)).toContain('columns=is_forbidden')
    const body = JSON.parse(String(init.body))
    expect(body.isForbidden).toBe(true)
    expect(body.isDeleted).toBeUndefined()
  })

  it('is idempotent on an already-forbidden user', async () => {
    await load()
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(
        json({ status: 'ok', data: { owner: 'archgate', name: 'op.a', isForbidden: true } }),
      )

    const r = await archguardAdmin.disableUser('op.a')
    expect(r.ok).toBe(true)
    expect(urlsCalled().some((u) => u.includes('update-user'))).toBe(false)
  })
})

describe('machine credential', () => {
  it('obtains a bearer through client_credentials, not a static secret', async () => {
    await load()
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(json({ status: 'ok', data: { name: 'g' } }))

    await archguardAdmin.ensureGroup('g')

    const [tokenUrl, tokenInit] = fetchMock.mock.calls[0]!
    expect(String(tokenUrl)).toContain('/api/login/oauth/access_token')
    expect(String(tokenInit.body)).toContain('grant_type=client_credentials')
    expect(new Headers(fetchMock.mock.calls[1]![1].headers).get('Authorization')).toBe(
      'Bearer bearer-abc',
    )
  })

  it('renews the bearer once on 401 and retries', async () => {
    await load()
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(new Response('', { status: 401 }))
      .mockResolvedValueOnce(json({ access_token: 'bearer-new', expires_in: 3600 }))
      .mockResolvedValueOnce(json({ status: 'ok', data: { name: 'g' } }))

    expect((await archguardAdmin.ensureGroup('g')).action).toBe('exists')
    expect(new Headers(fetchMock.mock.calls[3]![1].headers).get('Authorization')).toBe(
      'Bearer bearer-new',
    )
  })

  it('renews at most once per request — no renewal loop', async () => {
    await load()
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(new Response('', { status: 401 })) // get-group
      .mockResolvedValueOnce(json({ access_token: 'bearer-new', expires_in: 3600 }))
      .mockResolvedValueOnce(new Response('', { status: 401 })) // retry, still 401
      .mockResolvedValueOnce(json({ status: 'error', msg: 'denied' })) // add-group

    const r = await archguardAdmin.ensureGroup('g')
    expect(r.action).toBe('error')

    // A persistent 401 must not keep minting tokens: exactly one initial grant
    // plus one renewal, no matter how many requests follow.
    const grants = urlsCalled().filter((u) => u.includes('access_token'))
    expect(grants).toHaveLength(2)
  })
})
