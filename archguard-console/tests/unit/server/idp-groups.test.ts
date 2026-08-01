// Group names arrive in a different shape from each IdP. Getting this wrong
// silently empties a tenant's catalog, so it is pinned here.
//
//   Kanidm     archguard_users@id.archgate.com.br
//   ArchGuard  archgate/archguard_users

import { describe, expect, it } from 'vitest'
import {
  canonicalTenantKey,
  normalizeGroupName,
  normalizeGroupNames,
} from '@/server/idp/groups'

describe('normalizeGroupName', () => {
  it('strips the Kanidm @domain suffix', () => {
    expect(normalizeGroupName('archguard_users@id.archgate.com.br')).toBe(
      'archguard_users',
    )
  })

  it('strips the ArchGuard <org>/ prefix', () => {
    expect(normalizeGroupName('archgate/archguard_users')).toBe('archguard_users')
  })

  it('leaves a bare name untouched', () => {
    expect(normalizeGroupName('tenant_rio_quality')).toBe('tenant_rio_quality')
  })

  it('handles both shapes at once', () => {
    expect(normalizeGroupName('archgate/tenant_rio_quality@example.org')).toBe(
      'tenant_rio_quality',
    )
  })

  it('keeps only the last path segment on nested groups', () => {
    expect(normalizeGroupName('archgate/parent/child')).toBe('child')
  })

  it('trims whitespace', () => {
    expect(normalizeGroupName('  archgate/ops  ')).toBe('ops')
  })
})

describe('normalizeGroupNames', () => {
  it('normalizes a mixed-IdP claim array', () => {
    expect(
      normalizeGroupNames([
        'archgate/archguard_users',
        'tenant_rio_quality@id.archgate.com.br',
        'archguard_viewers',
      ]),
    ).toEqual(['archguard_users', 'tenant_rio_quality', 'archguard_viewers'])
  })

  it('drops Kanidm UUID entries and empties', () => {
    expect(
      normalizeGroupNames([
        '4f2b9c1a-1111-2222-3333-444455556666',
        'archgate/',
        'archguard_users',
      ]),
    ).toEqual(['archguard_users'])
  })

  it('returns empty for a missing claim', () => {
    expect(normalizeGroupNames(undefined)).toEqual([])
  })
})

describe('canonicalTenantKey', () => {
  it('reconciles the Warpgate role spelling with the IdP group', () => {
    expect(canonicalTenantKey('tenant-rio-quality')).toBe('tenant_rio_quality')
    expect(canonicalTenantKey('archgate/tenant_rio_quality')).toBe(
      'tenant_rio_quality',
    )
    expect(canonicalTenantKey('tenant_rio_quality@id.archgate.com.br')).toBe(
      'tenant_rio_quality',
    )
  })

  it('maps every shape of the same tenant to one key', () => {
    const shapes = [
      'tenant-rio-quality',
      'tenant_rio_quality',
      'archgate/tenant_rio_quality',
      'tenant_rio_quality@id.archgate.com.br',
      'archgate/tenant-rio-quality',
    ]
    expect(new Set(shapes.map(canonicalTenantKey)).size).toBe(1)
  })
})
