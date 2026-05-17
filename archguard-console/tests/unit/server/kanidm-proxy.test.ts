import { describe, it, expect } from 'vitest'
import { isAllowedPath } from '@/server/kanidm-proxy'

describe('kanidm-proxy isAllowedPath (SSRF allowlist)', () => {
  describe('accepted paths', () => {
    it.each([
      '/v1/person',
      '/v1/person/',
      '/v1/person/alice',
      '/v1/person/alice/_attr/mail',
      '/v1/group',
      '/v1/group/admins/_attr/member',
      '/v1/oauth2',
      '/v1/oauth2/foo/_basic_secret',
      '/v1/service_account',
      '/v1/service_account/sa/_api_token',
      '/v1/domain',
      '/v1/domain/_attr/domain_display_name',
      '/v1/system',
      '/v1/recycle_bin',
      '/v1/recycle_bin/uuid/_revive',
      '/status',
    ])('allows %s', (path) => {
      expect(isAllowedPath(path)).toBe(true)
    })
  })

  describe('rejected paths', () => {
    it.each([
      '/admin',
      '/v1',
      '/v1/',
      '/v1/persona',                  // person prefix-collision
      '/v1/persons',                  // plural, not in allowlist
      '/v1/groupings',                // group prefix-collision
      '/v2/person',
      '/internal/admin',
      '',
      '/',
      '/v1/person/../admin',          // traversal
      '/v1/person/..',
      '..',
    ])('rejects %s', (path) => {
      expect(isAllowedPath(path)).toBe(false)
    })
  })

  describe('normalization quirks', () => {
    it('collapses repeated slashes before matching', () => {
      expect(isAllowedPath('//v1//person')).toBe(true)
      expect(isAllowedPath('/v1//group/admins')).toBe(true)
    })

    it('strips a trailing slash before matching', () => {
      expect(isAllowedPath('/v1/person/')).toBe(true)
      expect(isAllowedPath('/status/')).toBe(true)
    })
  })
})
