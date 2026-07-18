import { describe, it, expect } from 'vitest'
import {
  normalizePerson,
  normalizeGroup,
  normalizeOAuth2Client,
  normalizeServiceAccount,
  normalizeCredentialStatus,
  parseScopeMaps,
  parseClaimMaps,
  extractTenantPrefix,
} from '@/lib/api/normalizers'
import type { KanidmEntry } from '@/lib/api/types/kanidm'

const personEntry: KanidmEntry = {
  attrs: {
    uuid: ['11111111-1111-1111-1111-111111111111'],
    name: ['alice'],
    displayname: ['Alice Liddell'],
    legalname: ['Alice P. Liddell'],
    mail: ['alice@example.com', 'a.liddell@example.com'],
    memberof: ['idm_all_persons', 'acme_users'],
    class: ['account', 'person', 'object'],
    ssh_publickey: ['ssh-ed25519 AAAA...'],
  },
}

const expiredPerson: KanidmEntry = {
  attrs: {
    uuid: ['22222222-2222-2222-2222-222222222222'],
    name: ['bob'],
    displayname: ['Bob'],
    class: ['person'],
    account_expire: ['2020-01-01T00:00:00Z'],
  },
}

const futurePerson: KanidmEntry = {
  attrs: {
    uuid: ['33333333-3333-3333-3333-333333333333'],
    name: ['carol'],
    displayname: ['Carol'],
    class: ['person'],
    account_valid_from: ['9999-01-01T00:00:00Z'],
  },
}

describe('normalizePerson', () => {
  it('maps Kanidm person fields to the Person model', () => {
    const p = normalizePerson(personEntry)
    expect(p.id).toBe('11111111-1111-1111-1111-111111111111')
    expect(p.username).toBe('alice')
    expect(p.displayName).toBe('Alice Liddell')
    expect(p.legalName).toBe('Alice P. Liddell')
    expect(p.emails).toEqual(['alice@example.com', 'a.liddell@example.com'])
    expect(p.groups).toEqual(['idm_all_persons', 'acme_users'])
    expect(p.classes).toContain('person')
    expect(p.sshKeys).toHaveLength(1)
    expect(p.status).toBe('active')
  })

  it('marks expired accounts when account_expire is in the past', () => {
    expect(normalizePerson(expiredPerson).status).toBe('expired')
  })

  it('marks not_yet_valid when account_valid_from is in the future', () => {
    expect(normalizePerson(futurePerson).status).toBe('not_yet_valid')
  })

  it('handles missing optional attributes safely', () => {
    const p = normalizePerson({
      attrs: { uuid: ['x'], name: ['y'], displayname: ['z'] },
    })
    expect(p.emails).toEqual([])
    expect(p.sshKeys).toEqual([])
    expect(p.groups).toEqual([])
    expect(p.legalName).toBeUndefined()
  })
})

describe('normalizeGroup', () => {
  it('maps name, description, members and detects tenant prefix', () => {
    const g = normalizeGroup({
      attrs: {
        uuid: ['g-1'],
        name: ['acme_admins'],
        description: ['Admins for acme tenant'],
        member: ['u-1', 'u-2'],
        class: ['group'],
      },
    })
    expect(g.id).toBe('g-1')
    expect(g.name).toBe('acme_admins')
    expect(g.description).toBe('Admins for acme tenant')
    expect(g.memberCount).toBe(2)
    expect(g.tenantName).toBe('acme')
    expect(g.isBuiltin).toBe(false)
  })

  it('marks builtin groups as such', () => {
    const g = normalizeGroup({
      attrs: { uuid: ['g'], name: ['idm_admins'], class: ['group'] },
    })
    expect(g.isBuiltin).toBe(true)
    expect(g.tenantName).toBeNull()
  })

  it('marks system_protected as builtin', () => {
    const g = normalizeGroup({
      attrs: {
        uuid: ['g'],
        name: ['custom_protected'],
        class: ['group', 'system_protected'],
      },
    })
    expect(g.isBuiltin).toBe(true)
  })
})

describe('normalizeOAuth2Client', () => {
  it('treats oauth2_resource_server_basic as basic confidential', () => {
    const c = normalizeOAuth2Client({
      attrs: {
        uuid: ['c-1'],
        name: ['app'],
        displayname: ['My App'],
        class: ['oauth2_resource_server', 'oauth2_resource_server_basic'],
        oauth2_rs_origin_landing: ['https://app.example.com'],
        oauth2_rs_origin: ['https://app.example.com/callback'],
      },
    })
    expect(c.type).toBe('basic')
    expect(c.hasSecret).toBe(true)
    expect(c.landingUrl).toBe('https://app.example.com')
    expect(c.redirectUrls).toContain('https://app.example.com/callback')
  })

  it('treats non-basic clients as public', () => {
    const c = normalizeOAuth2Client({
      attrs: {
        uuid: ['c-2'],
        name: ['app'],
        displayname: ['SPA'],
        class: ['oauth2_resource_server'],
        oauth2_rs_origin_landing: ['https://spa.example.com'],
      },
    })
    expect(c.type).toBe('public')
    expect(c.hasSecret).toBe(false)
  })
})

describe('normalizeServiceAccount', () => {
  it('maps service-account fields and starts with empty token list', () => {
    const sa = normalizeServiceAccount({
      attrs: {
        uuid: ['sa-1'],
        name: ['ci-bot'],
        displayname: ['CI Bot'],
        description: ['Used by CI'],
        memberof: ['archguard_service_accounts'],
      },
    })
    expect(sa.id).toBe('sa-1')
    expect(sa.name).toBe('ci-bot')
    expect(sa.description).toBe('Used by CI')
    expect(sa.groups).toEqual(['archguard_service_accounts'])
    expect(sa.apiTokens).toEqual([])
    expect(sa.status).toBe('active')
  })
})

describe('normalizeCredentialStatus', () => {
  it('returns all-false defaults for null/empty input', () => {
    const s = normalizeCredentialStatus(null)
    expect(s.hasPassword).toBe(false)
    expect(s.primaryMethod).toBe('none')
  })

  it('detects password-only as primaryMethod', () => {
    const s = normalizeCredentialStatus({
      creds: [{ type: 'password' }],
    })
    expect(s.hasPassword).toBe(true)
    expect(s.primaryMethod).toBe('password_only')
  })

  it('upgrades primaryMethod to password_mfa with TOTP', () => {
    const s = normalizeCredentialStatus({
      creds: [{ type: 'password' }, { type: 'totp' }],
    })
    expect(s.primaryMethod).toBe('password_mfa')
  })

  it('passkey wins over password', () => {
    const s = normalizeCredentialStatus({
      creds: [{ type: 'password' }, { type: 'passkey' }],
    })
    expect(s.primaryMethod).toBe('passkey')
    expect(s.hasPasskeys).toBe(true)
  })
})

describe('parseScopeMaps', () => {
  it('parses "uuid:scope1,scope2" entries', () => {
    const maps = parseScopeMaps([
      'g-1:openid,profile',
      'g-2:email',
    ])
    expect(maps).toEqual([
      { groupId: 'g-1', groupName: '', scopes: ['openid', 'profile'] },
      { groupId: 'g-2', groupName: '', scopes: ['email'] },
    ])
  })

  it('handles malformed entries without throwing', () => {
    const maps = parseScopeMaps(['no-colon-here'])
    expect(maps[0]?.groupId).toBe('no-colon-here')
    expect(maps[0]?.scopes).toEqual([])
  })
})

describe('parseClaimMaps', () => {
  it('parses "claim:uuid:val1,val2" entries', () => {
    const maps = parseClaimMaps([
      'department:g-1:engineering,security',
    ])
    expect(maps[0]).toEqual({
      claimName: 'department',
      groupId: 'g-1',
      groupName: '',
      values: ['engineering', 'security'],
    })
  })
})

describe('extractTenantPrefix', () => {
  it.each([
    ['acme_admins', 'acme'],
    ['acme_users', 'acme'],
    ['acme_service_desk', 'acme'],
    ['acme_developers', 'acme'],
    ['globex', 'globex'], // standalone tenant root
    ['tenant_rio_quality', 'tenant_rio_quality'], // ArchGate
    ['tenant_grupo_marra', 'tenant_grupo_marra'],
    // SPN form from Kanidm memberof
    [
      'tenant_rio_quality@id.archgate.com.br',
      'tenant_rio_quality',
    ],
  ])('returns tenant for %s', (group, tenant) => {
    expect(extractTenantPrefix(group)).toBe(tenant)
  })

  it.each([
    'idm_admins',
    'idm_people_admins',
    'system_admins',
    'archguard_admins',
    'archguard_super_admins',
    'archguard_users',
    'idm_all_persons',
    'archgate_tenants',
  ])('returns null for platform %s', (group) => {
    expect(extractTenantPrefix(group)).toBeNull()
  })

  it('returns null for ambiguous compound names', () => {
    expect(extractTenantPrefix('acme_custom_role')).toBeNull()
  })
})
