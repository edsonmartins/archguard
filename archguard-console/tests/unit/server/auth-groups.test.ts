import { describe, it, expect } from 'vitest'
import { normalizeGroups } from '@/server/auth'

describe('normalizeGroups (Kanidm groups claim cleanup)', () => {
  it('strips @domain SPN suffix', () => {
    expect(
      normalizeGroups([
        'archguard_admins@idm.example.com',
        'idm_service_desk@idm.example.com',
      ]),
    ).toEqual(['archguard_admins', 'idm_service_desk'])
  })

  it('filters out UUID entries', () => {
    expect(
      normalizeGroups([
        '11111111-2222-3333-4444-555555555555',
        'archguard_admins',
      ]),
    ).toEqual(['archguard_admins'])
  })

  it('keeps already-normalized group names untouched', () => {
    expect(normalizeGroups(['acme_users'])).toEqual(['acme_users'])
  })

  it('strips suffix and filters UUID in the same payload', () => {
    expect(
      normalizeGroups([
        'archguard_admins@localhost',
        'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee',
        'idm_admins@localhost',
      ]),
    ).toEqual(['archguard_admins', 'idm_admins'])
  })

  it('returns an empty array for empty input', () => {
    expect(normalizeGroups([])).toEqual([])
  })

  it('handles multiple @ characters by stripping at the first', () => {
    expect(normalizeGroups(['weird@name@host'])).toEqual(['weird'])
  })
})
