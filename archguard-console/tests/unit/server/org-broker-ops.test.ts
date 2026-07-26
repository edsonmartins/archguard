import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests } from '../../../src/server/db'
import {
  friendlySecretBackendError,
  getOrgBrokerSettings,
  saveOrgBrokerSettings,
} from '../../../src/server/org-broker-ops'
import { resolveCheckoutWebhookUrl } from '../../../src/server/org-broker-ops'

describe('org-broker-ops settings', () => {
  let dir: string
  const prevEnv = process.env.ORG_CHECKOUT_WEBHOOK_URL

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), 'org-broker-'))
    _resetDbForTests(join(dir, 't.sqlite'))
    delete process.env.ORG_CHECKOUT_WEBHOOK_URL
  })

  afterEach(() => {
    _resetDbForTests()
    rmSync(dir, { recursive: true, force: true })
    if (prevEnv === undefined) delete process.env.ORG_CHECKOUT_WEBHOOK_URL
    else process.env.ORG_CHECKOUT_WEBHOOK_URL = prevEnv
  })

  it('saves webhook and TTL via Manager settings (no env)', () => {
    const s = saveOrgBrokerSettings(
      {
        checkout_webhook_url: 'https://hooks.example.com/x',
        ttl_default_seconds: 1800,
        ttl_max_seconds: 7200,
      },
      'admin@test',
    )
    expect(s.checkout_webhook_url).toBe('https://hooks.example.com/x')
    expect(s.ttl_default_seconds).toBe(1800)
    expect(s.ttl_max_seconds).toBe(7200)
    expect(s.webhook_from_env).toBe(false)
    expect(resolveCheckoutWebhookUrl()).toBe('https://hooks.example.com/x')
  })

  it('rejects non-HTTPS webhook', () => {
    expect(() =>
      saveOrgBrokerSettings(
        { checkout_webhook_url: 'http://insecure.example/x' },
        'admin',
      ),
    ).toThrow(/HTTPS/)
  })

  it('falls back to env webhook when Manager setting empty', () => {
    process.env.ORG_CHECKOUT_WEBHOOK_URL = 'https://env.example/hook'
    const s = getOrgBrokerSettings()
    expect(s.checkout_webhook_url).toBe('https://env.example/hook')
    expect(s.webhook_from_env).toBe(true)
  })

  it('maps backend errors to console-facing messages', () => {
    expect(friendlySecretBackendError('permission denied')).toMatch(/permissão/i)
    expect(friendlySecretBackendError('OpenBao write 404: no handler')).toMatch(
      /Path|path|indisponível/i,
    )
    expect(friendlySecretBackendError('sealed')).toMatch(/selado/i)
  })
})
