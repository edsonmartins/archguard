// tests/e2e/unified-bff.spec.ts
//
// Contract tests for the operator BFF (/api/unified/v1/*) against a running
// console. This is the surface the 2026-07-28 audit found reachable from the
// internet with a forged `Bearer lab-<user>` (P0-1), and answering CORS to any
// origin with credentials. Both are asserted here so a regression fails the
// suite instead of shipping.
//
// No browser session is used on purpose: these run as an anonymous client.

import { test, expect } from '@playwright/test'

const ENDPOINTS = [
  '/api/unified/v1/connections',
  '/api/unified/v1/operator/bootstrap',
] as const

test.describe('unified BFF — authentication', () => {
  for (const path of ENDPOINTS) {
    test(`${path} rejects an anonymous caller`, async ({ request }) => {
      const res = await request.get(path, { failOnStatusCode: false })
      expect(res.status()).toBe(401)
    })

    test(`${path} rejects a forged lab bearer`, async ({ request }) => {
      const res = await request.get(path, {
        headers: { Authorization: 'Bearer lab-anyone' },
        failOnStatusCode: false,
      })
      expect(res.status()).toBe(401)
    })

    test(`${path} rejects a lab bearer with a wrong secret`, async ({
      request,
    }) => {
      const res = await request.get(path, {
        headers: { Authorization: 'Bearer lab-anyone:not-the-secret' },
        failOnStatusCode: false,
      })
      expect(res.status()).toBe(401)
    })
  }

  test('POST /sessions rejects an anonymous caller', async ({ request }) => {
    const res = await request.post('/api/unified/v1/sessions', {
      data: { target: 'rio-lab-ssh', protocol: 'ssh' },
      failOnStatusCode: false,
    })
    expect(res.status()).toBe(401)
  })

  test('POST /sessions rejects a forged lab bearer', async ({ request }) => {
    const res = await request.post('/api/unified/v1/sessions', {
      headers: { Authorization: 'Bearer lab-anyone' },
      data: { target: 'rio-lab-ssh', protocol: 'ssh' },
      failOnStatusCode: false,
    })
    expect(res.status()).toBe(401)
  })

  test('a rejected call never returns catalog or tunnel material', async ({
    request,
  }) => {
    const res = await request.get('/api/unified/v1/connections', {
      headers: { Authorization: 'Bearer lab-anyone' },
      failOnStatusCode: false,
    })
    const body = await res.text()
    expect(body).not.toMatch(/tunnel_url|connect_data|connections/)
  })
})

test.describe('unified BFF — CORS', () => {
  test('does not reflect an arbitrary origin', async ({ request }) => {
    const res = await request.fetch('/api/unified/v1/connections', {
      method: 'OPTIONS',
      headers: { Origin: 'https://evil.example' },
      failOnStatusCode: false,
    })
    const allow = res.headers()['access-control-allow-origin']
    expect(allow).not.toBe('https://evil.example')
    expect(allow).not.toBe('*')
  })

  test('does not allow client-asserted identity headers', async ({
    request,
  }) => {
    const res = await request.fetch('/api/unified/v1/connections', {
      method: 'OPTIONS',
      headers: { Origin: 'https://evil.example' },
      failOnStatusCode: false,
    })
    const allowed = res.headers()['access-control-allow-headers'] ?? ''
    expect(allowed.toLowerCase()).not.toContain('x-archgate-user')
    expect(allowed.toLowerCase()).not.toContain('x-archgate-tenants')
  })
})
