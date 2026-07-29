import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _resetDbForTests } from '../../../src/server/db'
import {
  backfillOrgAccountFederation,
  deleteOrgAccount,
  getOrgAccount,
  listOrgAccounts,
  seedDefaultOrgAccountsIfEmpty,
  upsertOrgAccount,
} from '../../../src/server/org-accounts'

describe('org-accounts persistence', () => {
  let dir: string

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), 'org-acc-'))
    _resetDbForTests(join(dir, 't.sqlite'))
  })

  afterEach(() => {
    _resetDbForTests()
    rmSync(dir, { recursive: true, force: true })
  })

  it('seeds defaults once and never stores password fields', () => {
    const n = seedDefaultOrgAccountsIfEmpty('test')
    expect(n).toBeGreaterThan(5)
    expect(seedDefaultOrgAccountsIfEmpty('test')).toBe(0)
    const all = listOrgAccounts()
    expect(all.some((a) => a.slug === 'apple-appstore-integrall')).toBe(true)
    expect(all.every((a) => a.criticality === 'P0' ? a.requires_dual_control : true || true)).toBe(
      true,
    )
    const apple = getOrgAccount('apple-appstore-integrall')!
    expect(apple.requires_dual_control).toBe(true)
    expect(JSON.stringify(apple)).not.toMatch(/password_value|secret_value/)
    expect(apple.secret_ref).toContain('secret/data/org/')
  })

  it('upserts and deletes by slug', () => {
    upsertOrgAccount(
      {
        slug: 'demo-saas',
        name: 'Demo SaaS',
        category: 'vendor',
        criticality: 'P2',
      },
      'admin@test',
    )
    const got = getOrgAccount('demo-saas')
    expect(got?.name).toBe('Demo SaaS')
    expect(deleteOrgAccount('demo-saas')).toBe(true)
    expect(getOrgAccount('demo-saas')).toBeNull()
  })

  it('backfills federation_status on known seed slugs still password_only', () => {
    upsertOrgAccount(
      {
        slug: 'vendax-admin',
        name: 'Vendax · admin',
        category: 'product_admin',
        product: 'vendax',
        criticality: 'P1',
        auth_kind: 'password',
        federation_status: 'password_only',
        secret_ref: 'secret/data/org/product/vendax-admin',
      },
      'legacy',
    )
    const n = backfillOrgAccountFederation('migrate')
    expect(n).toBeGreaterThanOrEqual(1)
    const v = getOrgAccount('vendax-admin')!
    expect(v.federation_status).toBe('oidc_primary')
    expect(v.oidc_client_id).toBe('vendax-admin')
    expect(v.auth_kind).toBe('oidc')
    // second pass is no-op for this slug
    expect(backfillOrgAccountFederation('migrate')).toBe(0)
  })

  // Regression (audit 2026-07-28): when only the client id was missing, the
  // backfill omitted `name` from the upsert, which throws "name required".
  it('backfills a missing client id without losing name or category', () => {
    upsertOrgAccount(
      {
        slug: 'vendax-admin',
        name: 'Vendax · admin',
        category: 'product_admin',
        product: 'vendax',
        criticality: 'P1',
        auth_kind: 'oidc',
        // Already federated, so only the client id is missing.
        federation_status: 'oidc_primary',
      },
      'legacy',
    )

    expect(() => backfillOrgAccountFederation('migrate')).not.toThrow()

    const v = getOrgAccount('vendax-admin')!
    expect(v.oidc_client_id).toBe('vendax-admin')
    expect(v.name).toBe('Vendax · admin')
    expect(v.category).toBe('product_admin')
    expect(v.federation_status).toBe('oidc_primary')
  })

  it('does not overwrite explicit oidc_only federation', () => {
    upsertOrgAccount(
      {
        slug: 'vendax-admin',
        name: 'Vendax',
        category: 'product_admin',
        product: 'vendax',
        criticality: 'P1',
        auth_kind: 'oidc',
        federation_status: 'oidc_only',
        oidc_client_id: 'custom-client',
      },
      'admin',
    )
    expect(backfillOrgAccountFederation('migrate')).toBe(0)
    const v = getOrgAccount('vendax-admin')!
    expect(v.federation_status).toBe('oidc_only')
    expect(v.oidc_client_id).toBe('custom-client')
  })
})
