// The orchestration API now authenticates every call. The console must present
// its service token on every outbound call, applied centrally so no call site
// can forget it (audit 2026-07-28, P0-3).

import { describe, expect, it, beforeEach, vi, afterEach } from 'vitest'

vi.mock('@/server/logger', () => ({
  logger: { warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() },
}))

let integrationFetch: typeof import('@/server/http-integration-client').integrationFetch

const fetchMock = vi.fn(async () => new Response('{}', { status: 200 }))

async function load(env: Record<string, string | undefined>) {
  vi.resetModules()
  for (const [k, v] of Object.entries(env)) {
    if (v === undefined) delete process.env[k]
    else process.env[k] = v
  }
  ;({ integrationFetch } = await import('@/server/http-integration-client'))
}

function sentHeaders(): Headers {
  const call = fetchMock.mock.calls.at(-1) as unknown as [string, RequestInit]
  return new Headers(call[1].headers)
}

beforeEach(() => {
  fetchMock.mockClear()
  vi.stubGlobal('fetch', fetchMock)
  delete process.env.ORCH_API_TOKEN
  delete process.env.AGENT_API_TOKEN
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('outbound service auth', () => {
  it('sends the orchestration token as a bearer', async () => {
    await load({ ORCH_API_TOKEN: 'orch-secret' })
    await integrationFetch('http://orch:8090/orchestration/v1/tenants/sync', {
      method: 'POST',
      integration: 'orchestration',
      headers: { 'Content-Type': 'application/json' },
    })
    expect(sentHeaders().get('Authorization')).toBe('Bearer orch-secret')
    expect(sentHeaders().get('Content-Type')).toBe('application/json')
  })

  it('sends the agent-control token as a bearer', async () => {
    await load({ AGENT_API_TOKEN: 'agent-secret' })
    await integrationFetch('http://agent:8091/agentic/v1/actions', {
      method: 'POST',
      integration: 'agent-control',
    })
    expect(sentHeaders().get('Authorization')).toBe('Bearer agent-secret')
  })

  it('adds nothing when no token is configured', async () => {
    await load({})
    await integrationFetch('http://orch:8090/orchestration/v1/tenants/sync', {
      integration: 'orchestration',
    })
    expect(sentHeaders().get('Authorization')).toBeNull()
  })

  it('never leaks the token to an unrelated integration', async () => {
    await load({ ORCH_API_TOKEN: 'orch-secret' })
    await integrationFetch('https://wg.archgate.com.br/@warpgate/admin/api/users', {
      integration: 'warpgate',
    })
    expect(sentHeaders().get('Authorization')).toBeNull()
  })

  it('does not override an explicit Authorization header', async () => {
    await load({ ORCH_API_TOKEN: 'orch-secret' })
    await integrationFetch('http://orch:8090/orchestration/v1/tenants/sync', {
      integration: 'orchestration',
      headers: { Authorization: 'Bearer caller-supplied' },
    })
    expect(sentHeaders().get('Authorization')).toBe('Bearer caller-supplied')
  })
})
