import { describe, expect, it } from 'vitest'
import {
  getGuiaByRota,
  getModulosPorFase,
  guiaModulos,
} from '@/lib/guia/guia-modulos'

describe('getGuiaByRota', () => {
  it('matches exact routes', () => {
    expect(getGuiaByRota('/dashboard')?.modulo).toBe('AG-HOME')
    expect(getGuiaByRota('/org-accounts')?.modulo).toBe('AG-OCB')
    expect(getGuiaByRota('/sites')?.titulo).toMatch(/Sites|Clientes/i)
  })

  it('inherits parent guide for detail paths', () => {
    expect(getGuiaByRota('/identities/abc-123')?.modulo).toBe('AG-ID')
    expect(getGuiaByRota('/sites/rio-lab/edit')?.modulo).toBe('AG-SITE')
    expect(getGuiaByRota('/oauth2/vendax-admin')?.modulo).toBe('AG-OIDC')
    expect(getGuiaByRota('/service-accounts/sa-1')?.modulo).toBe('AG-SA')
  })

  it('covers all main nav modules', () => {
    const required = [
      '/dashboard',
      '/identities',
      '/service-accounts',
      '/groups',
      '/oauth2',
      '/vault',
      '/sites',
      '/gateways',
      '/secrets',
      '/org-accounts',
      '/oracle',
      '/platform',
      '/integrations/mentors-axis',
      '/audit',
      '/recycle-bin',
      '/settings',
    ]
    for (const r of required) {
      expect(getGuiaByRota(r), `missing guide for ${r}`).toBeTruthy()
    }
  })

  it('groups by fase', () => {
    const byFase = getModulosPorFase()
    expect(Object.keys(byFase).length).toBeGreaterThan(2)
    expect(guiaModulos.length).toBeGreaterThanOrEqual(14)
  })
})
