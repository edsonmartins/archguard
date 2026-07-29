// Guards the lab-bearer hardening from the 2026-07-28 audit (P0-1).
//
// A `Bearer lab-<user>` with no shared secret must never mint a session, and a
// lab session must never inherit a real client tenant by default.

import { describe, expect, it, beforeEach, vi } from 'vitest'

vi.mock('@/server/unified-bff', () => ({
  requireUnifiedSession: () => {
    throw new Error('Unauthorized')
  },
}))

vi.mock('@/server/logger', () => ({
  logger: { warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() },
}))

let labSession: typeof import('@/server/operator-session').labSession

async function load(env: Record<string, string | undefined>) {
  vi.resetModules()
  for (const [k, v] of Object.entries(env)) {
    if (v === undefined) delete process.env[k]
    else process.env[k] = v
  }
  ;({ labSession } = await import('@/server/operator-session'))
}

beforeEach(() => {
  delete process.env.ARCHGATE_CONNECT_LAB
  delete process.env.ARCHGATE_CONNECT_LAB_TOKEN
  delete process.env.ARCHGATE_CONNECT_LAB_GROUPS
})

describe('lab bearer', () => {
  it('is refused when the lab path is off', async () => {
    await load({ ARCHGATE_CONNECT_LAB: '0', ARCHGATE_CONNECT_LAB_TOKEN: 's3cr3t' })
    expect(labSession('lab-op-a:s3cr3t')).toBeNull()
  })

  it('is refused when the flag is on but no secret is configured', async () => {
    await load({ ARCHGATE_CONNECT_LAB: '1', ARCHGATE_CONNECT_LAB_TOKEN: undefined })
    expect(labSession('lab-op-a')).toBeNull()
    expect(labSession('lab-op-a:anything')).toBeNull()
  })

  it('is refused without the secret suffix', async () => {
    await load({ ARCHGATE_CONNECT_LAB: '1', ARCHGATE_CONNECT_LAB_TOKEN: 's3cr3t' })
    expect(labSession('lab-op-a')).toBeNull()
  })

  it('is refused with a wrong secret', async () => {
    await load({ ARCHGATE_CONNECT_LAB: '1', ARCHGATE_CONNECT_LAB_TOKEN: 's3cr3t' })
    expect(labSession('lab-op-a:wrong')).toBeNull()
    expect(labSession('lab-op-a:s3cr3tX')).toBeNull()
  })

  it('is refused with an empty username', async () => {
    await load({ ARCHGATE_CONNECT_LAB: '1', ARCHGATE_CONNECT_LAB_TOKEN: 's3cr3t' })
    expect(labSession('lab-:s3cr3t')).toBeNull()
  })

  it('accepts a correctly signed bearer', async () => {
    await load({ ARCHGATE_CONNECT_LAB: '1', ARCHGATE_CONNECT_LAB_TOKEN: 's3cr3t' })
    const s = labSession('lab-op-a:s3cr3t')
    expect(s?.isAuthenticated).toBe(true)
    expect(s?.isAdmin).toBe(false)
    expect(s?.user?.name).toBe('op-a')
  })

  it('does not grant a real client tenant by default', async () => {
    await load({ ARCHGATE_CONNECT_LAB: '1', ARCHGATE_CONNECT_LAB_TOKEN: 's3cr3t' })
    const s = labSession('lab-op-a:s3cr3t')
    expect(s?.groups).toEqual(['archguard_users'])
    expect(s?.groups.some((g) => g.startsWith('tenant_'))).toBe(false)
  })

  it('grants only the groups explicitly configured', async () => {
    await load({
      ARCHGATE_CONNECT_LAB: '1',
      ARCHGATE_CONNECT_LAB_TOKEN: 's3cr3t',
      ARCHGATE_CONNECT_LAB_GROUPS: 'archguard_users, tenant_lab',
    })
    expect(labSession('lab-op-a:s3cr3t')?.groups).toEqual([
      'archguard_users',
      'tenant_lab',
    ])
  })

  it('ignores non-lab bearers', async () => {
    await load({ ARCHGATE_CONNECT_LAB: '1', ARCHGATE_CONNECT_LAB_TOKEN: 's3cr3t' })
    expect(labSession('eyJhbGciOi.some.jwt')).toBeNull()
  })
})
