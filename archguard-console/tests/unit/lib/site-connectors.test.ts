import { describe, expect, it } from 'vitest'
import {
  normalizeSiteConnectors,
  primaryStackFromConnectors,
  type SiteConnector,
} from '@/lib/api/types/site'
import { parseSiteDocument, siteToYaml } from '@/lib/sites/site-yaml'
import type { Site } from '@/lib/api/types/site'

describe('multi-connector normalize', () => {
  it('synthesizes one connector from legacy fields', () => {
    const c = normalizeSiteConnectors({
      slug: 'rio_quality',
      tipo: 'tunnel_agent',
      stack: 'openfortivpn',
      connector_id: 'connector-rio-forti',
      subnets: ['10.1.0.0/16'],
      stack_meta: { forti_host: 'vpn.rio.example' },
      connectors: [],
    })
    expect(c).toHaveLength(1)
    expect(c[0]!.id).toBe('connector-rio-forti')
    expect(c[0]!.stack).toBe('openfortivpn')
    expect(c[0]!.subnets).toEqual(['10.1.0.0/16'])
  })

  it('keeps multi connectors and reports mixed stack', () => {
    const connectors: SiteConnector[] = [
      { id: 'c-forti', stack: 'openfortivpn', subnets: ['10.1.0.0/16'] },
      { id: 'c-ovpn', stack: 'openvpn', subnets: ['10.20.0.0/16'] },
    ]
    expect(primaryStackFromConnectors(connectors)).toBe('mixed')
    const n = normalizeSiteConnectors({
      slug: 'rio',
      tipo: 'tunnel_agent',
      stack: 'mixed',
      connector_id: 'c-forti',
      subnets: [],
      stack_meta: {},
      connectors,
    })
    expect(n).toHaveLength(2)
  })
})

describe('site yaml multi-connector + secret_ref', () => {
  it('round-trips connectors and secret_ref', () => {
    const sample: Site = {
      slug: 'rio_quality',
      cliente: 'Rio Quality',
      tenant_group: 'tenant_rio_quality',
      ambiente: 'producao',
      tipo: 'tunnel_agent',
      stack: 'mixed',
      connector_id: 'c-forti',
      subnets: ['10.1.0.0/16'],
      stack_meta: {},
      connectors: [
        {
          id: 'c-forti',
          stack: 'openfortivpn',
          subnets: ['10.1.0.0/16'],
          meta: { forti_host: 'vpn.rio' },
        },
        {
          id: 'c-aws',
          stack: 'openvpn',
          subnets: ['10.20.0.0/16'],
          meta: { ovpn_path: '/var/lib/archgate/connector/rio/aws.ovpn' },
        },
      ],
      targets: [
        {
          nome: 'rio-api-1',
          engine: 'warpgate',
          protocolo: 'ssh',
          host: '10.20.0.11',
          port: 22,
          roles: ['tenant-rio-quality'],
          secret_ref: 'secret/data/archgate/targets/rio-api-1',
          connector_id: 'c-aws',
        },
      ],
      warpgate_roles: ['tenant-rio-quality'],
      notas: '',
      inventariado: true,
      connector_deployed: false,
      smoke_operador: false,
      updated_at: '2026-07-17T00:00:00.000Z',
      updated_by: 'test',
    }
    const yaml = siteToYaml(sample)
    expect(yaml).toContain('c-aws')
    expect(yaml).toContain('secret_ref')
    const parsed = parseSiteDocument(yaml)
    expect(parsed.connectors).toHaveLength(2)
    expect(parsed.targets[0]!.secret_ref).toContain('rio-api-1')
    expect(parsed.targets[0]!.connector_id).toBe('c-aws')
  })
})
