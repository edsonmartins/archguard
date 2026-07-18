import { describe, expect, it } from 'vitest'
import {
  parseSiteDocument,
  siteToYaml,
} from '@/lib/sites/site-yaml'
import type { Site } from '@/lib/api/types/site'

const sample: Site = {
  slug: 'acme',
  cliente: 'ACME SA',
  tenant_group: 'tenant_acme',
  ambiente: 'producao',
  tipo: 'connector_peer',
  stack: 'wireguard',
  connector_id: 'connector-acme',
  subnets: ['10.1.0.0/24'],
  stack_meta: {
    host: 'vpn.acme.example',
    mfa: true,
    connector_checklist: { unit_systemd: true },
  },
  connectors: [],
  targets: [
    {
      nome: 'acme-ssh',
      engine: 'warpgate',
      protocolo: 'ssh',
      host: '10.1.0.10',
      port: 22,
      roles: ['tenant-acme'],
    },
  ],
  warpgate_roles: ['tenant-acme'],
  notas: 'ficha de teste',
  inventariado: true,
  connector_deployed: false,
  smoke_operador: false,
  updated_at: '2026-07-11T12:00:00.000Z',
  updated_by: 'tester',
}

describe('site-yaml (yaml package)', () => {
  it('round-trips site ficha', () => {
    const yaml = siteToYaml(sample)
    expect(yaml).toContain('slug: "acme"')
    const parsed = parseSiteDocument(yaml)
    expect(parsed.slug).toBe('acme')
    expect(parsed.cliente).toBe('ACME SA')
    expect(parsed.tenant_group).toBe('tenant_acme')
    expect(parsed.stack).toBe('wireguard')
    expect(parsed.subnets).toEqual(['10.1.0.0/24'])
    expect(parsed.stack_meta.host).toBe('vpn.acme.example')
    expect(parsed.stack_meta.mfa).toBe(true)
    expect(parsed.targets).toHaveLength(1)
    expect(parsed.targets[0]!.nome).toBe('acme-ssh')
    expect(parsed.inventariado).toBe(true)
  })

  it('parses JSON ficha', () => {
    const parsed = parseSiteDocument(
      JSON.stringify({
        slug: 'x',
        cliente: 'X',
        tenant_group: 'tenant_x',
        conectividade: { tipo: 'tunnel_agent', stack: 'openvpn' },
        targets: [],
      }),
    )
    expect(parsed.slug).toBe('x')
    expect(parsed.tipo).toBe('tunnel_agent')
    expect(parsed.stack).toBe('openvpn')
  })
})
