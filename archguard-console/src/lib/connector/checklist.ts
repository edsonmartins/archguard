// Connector onboarding checklist by stack (CP-5) — pure logic, client+server safe

import type { Site, SiteStack, SiteTipo } from '@/lib/api/types/site'
import {
  getConnectorChecklist,
  hasSecretRefMeta,
  hasVpnEndpointMeta,
  vpnEndpointLabel,
} from '@/lib/api/types/site'

export type ChecklistItemId =
  | 'stack_chosen'
  | 'tipo_chosen'
  | 'tenant_linked'
  | 'connector_id'
  | 'subnets'
  | 'vpn_endpoint_meta'
  | 'secret_ref'
  | 'targets_defined'
  | 'warpgate_synced'
  | 'connector_deployed_flag'
  | 'smoke_operador'
  | 'no_vpn_user_exception'
  | 'unit_systemd'
  | 'peer_keys_generated'

export type ChecklistItem = {
  id: ChecklistItemId
  label: string
  description: string
  /** How to satisfy — ops hint */
  how: string
  auto?: boolean
}

const COMMON: ChecklistItem[] = [
  {
    id: 'stack_chosen',
    label: 'Stack VPN definida',
    description: 'wireguard | openvpn | openfortivpn | cloudflare_tunnel | …',
    how: 'Editar site → Conectividade → Stack',
    auto: true,
  },
  {
    id: 'tipo_chosen',
    label: 'Tipo ADR-008 definido',
    description: 'connector_peer | tunnel_agent (evitar vpn_user_exception)',
    how: 'Editar site → Tipo',
    auto: true,
  },
  {
    id: 'tenant_linked',
    label: 'Tenant Kanidm ligado',
    description: 'Grupo tenant_* no site',
    how: 'Editar site → tenant_group',
    auto: true,
  },
  {
    id: 'connector_id',
    label: 'Connector ID',
    description: 'Identificador estável do path institucional',
    how: 'Editar site → connector_id',
    auto: true,
  },
  {
    id: 'subnets',
    label: 'Subnets alvo (least privilege)',
    description: 'Redes internas dos targets',
    how: 'Editar site → Subnets',
    auto: true,
  },
  {
    id: 'targets_defined',
    label: 'Targets inventariados',
    description: 'Pelo menos um target SSH/DB/RDP',
    how: 'Editar site → Targets',
    auto: true,
  },
  {
    id: 'warpgate_synced',
    label: 'Targets no Warpgate',
    description: 'Sincronizar Warpgate a partir do site',
    how: 'Botão Sincronizar Warpgate nesta página',
    auto: true,
  },
  {
    id: 'connector_deployed_flag',
    label: 'Connector em produção (flag)',
    description: 'Marcado após deploy real do peer/VPN',
    how: 'Editar site → estado “Connector em produção”',
    auto: true,
  },
  {
    id: 'smoke_operador',
    label: 'Smoke operador OK',
    description: 'SSO → bastion → target validado',
    how: 'Editar site → estado “Smoke operador”',
    auto: true,
  },
  {
    id: 'no_vpn_user_exception',
    label: 'Sem VPN no notebook do ops',
    description: 'Preferir path institucional',
    how: 'Tipo ≠ vpn_user_exception',
    auto: true,
  },
]

const BY_STACK: Partial<Record<SiteStack, ChecklistItem[]>> = {
  wireguard: [
    {
      id: 'peer_keys_generated',
      label: 'Chaves WG (host, fora do git)',
      description: 'Par de chaves em /var/lib/archgate/connector/<site>/',
      how: 'wg genkey | tee … ; ver wireguard-peer.example.conf',
    },
    {
      id: 'vpn_endpoint_meta',
      label: 'Endpoint peer documentado',
      description: 'host:port do peer do cliente',
      how: 'stack_meta.endpoint ou host no formulário',
      auto: true,
    },
    {
      id: 'unit_systemd',
      label: 'Interface WG no connector',
      description: 'wg-quick ou container NET_ADMIN',
      how: 'deploy-wg-piloto-lab.sh (lab) ou peer produção',
    },
  ],
  openvpn: [
    {
      id: 'secret_ref',
      label: 'Profile OpenVPN no host',
      description: 'Path do .ovpn (sem colar senha no console)',
      how: 'stack_meta.profile_ref',
      auto: true,
    },
    {
      id: 'unit_systemd',
      label: 'Unit systemd OpenVPN',
      description: 'archgate-connector-openvpn@<site>',
      how: 'stacks/openvpn/systemd/',
    },
    {
      id: 'vpn_endpoint_meta',
      label: 'Host VPN / portal documentado',
      how: 'stack_meta.host',
      description: 'Endpoint do servidor OpenVPN do cliente',
      auto: true,
    },
  ],
  openfortivpn: [
    {
      id: 'vpn_endpoint_meta',
      label: 'Host Forti SSL documentado',
      description: 'vpn.cliente:443',
      how: 'stack_meta.host',
      auto: true,
    },
    {
      id: 'secret_ref',
      label: 'Conf openfortivpn no host',
      description: '/var/lib/archgate/connector/<site>/forti/',
      how: 'stacks/openfortivpn/ — conta de serviço',
    },
    {
      id: 'unit_systemd',
      label: 'Unit systemd Forti',
      description: 'archgate-connector-forti@<site>',
      how: 'stacks/openfortivpn/systemd/',
    },
  ],
  cloudflare_tunnel: [
    {
      id: 'vpn_endpoint_meta',
      label: 'Tunnel name documentado',
      description: 'Nome do tunnel Cloudflare',
      how: 'stack_meta.tunnel_name ou host',
      auto: true,
    },
  ],
  lab_overlay: [
    {
      id: 'unit_systemd',
      label: 'Piloto lab ativo',
      description: 'Serviços piloto connector no Swarm',
      how: 'deploy-stack.sh wireguard|openvpn',
    },
  ],
}

export function checklistForSite(site: Pick<Site, 'stack' | 'tipo'>): ChecklistItem[] {
  const stackItems = BY_STACK[site.stack] || []
  return [...COMMON, ...stackItems]
}

export type ChecklistEval = {
  id: ChecklistItemId
  label: string
  description: string
  how: string
  done: boolean
  source: 'auto' | 'manual' | 'probe'
  detail?: string
}

function manualMap(site: Site): Record<string, boolean> {
  return getConnectorChecklist(site.stack_meta)
}

export function evaluateChecklist(site: Site): ChecklistEval[] {
  const manual = manualMap(site)
  const items = checklistForSite(site)
  const meta = site.stack_meta || {}

  return items.map((item) => {
    let done = false
    let source: ChecklistEval['source'] = item.auto ? 'auto' : 'manual'
    let detail: string | undefined

    switch (item.id) {
      case 'stack_chosen':
        done = site.stack !== 'a_confirmar'
        detail = site.stack
        break
      case 'tipo_chosen':
        done = site.tipo !== 'a_confirmar'
        detail = site.tipo
        break
      case 'tenant_linked':
        done = !!site.tenant_group?.startsWith('tenant_')
        detail = site.tenant_group
        break
      case 'connector_id':
        done = !!site.connector_id?.trim()
        detail = site.connector_id
        break
      case 'subnets':
        done = (site.subnets || []).length > 0
        detail = site.subnets?.join(', ')
        break
      case 'targets_defined':
        done = (site.targets || []).length > 0
        detail = `${site.targets?.length || 0} target(s)`
        break
      case 'warpgate_synced':
        // inventariado often set after sync; or explicit meta
        done =
          !!site.inventariado ||
          !!manual.warpgate_synced ||
          !!meta.warpgate_synced
        break
      case 'connector_deployed_flag':
        done = !!site.connector_deployed
        break
      case 'smoke_operador':
        done = !!site.smoke_operador
        break
      case 'no_vpn_user_exception':
        done = site.tipo !== 'vpn_user_exception'
        detail = site.tipo
        break
      case 'vpn_endpoint_meta':
        done = hasVpnEndpointMeta(meta)
        detail = vpnEndpointLabel(meta)
        break
      case 'secret_ref':
        done = hasSecretRefMeta(meta)
        detail = String(
          meta.profile_ref || meta.secret_ref || meta.ovpn_path || '',
        )
        break
      case 'peer_keys_generated':
      case 'unit_systemd':
        done = !!manual[item.id]
        source = 'manual'
        break
      default:
        done = !!manual[item.id]
        source = 'manual'
    }

    // manual override can force true
    if (manual[item.id] === true) {
      done = true
      if (!item.auto) source = 'manual'
    }
    if (manual[item.id] === false && !item.auto) {
      done = false
    }

    return {
      id: item.id,
      label: item.label,
      description: item.description,
      how: item.how,
      done,
      source,
      detail,
    }
  })
}

export function checklistProgress(evals: ChecklistEval[]): {
  done: number
  total: number
  pct: number
} {
  const total = evals.length
  const done = evals.filter((e) => e.done).length
  return { done, total, pct: total ? Math.round((done / total) * 100) : 0 }
}

export function deployHints(site: Site): string[] {
  const id = site.connector_id || site.slug
  const lines: string[] = [
    `# Site ${site.cliente} (${site.slug}) stack=${site.stack} tipo=${site.tipo}`,
  ]
  switch (site.stack) {
    case 'wireguard':
      lines.push(
        `bash deploy/connector/deploy-stack.sh wireguard  # lab`,
        `# prod: keys em /var/lib/archgate/connector/${id}/ + wireguard-peer.example.conf`,
      )
      break
    case 'openvpn':
      lines.push(
        `bash deploy/connector/deploy-stack.sh openvpn  # lab`,
        `systemctl enable --now archgate-connector-openvpn@${id}`,
      )
      break
    case 'openfortivpn':
      lines.push(
        `# conf: /var/lib/archgate/connector/${id}/forti/openfortivpn.conf`,
        `systemctl enable --now archgate-connector-forti@${id}`,
      )
      break
    case 'lab_overlay':
      lines.push(
        `bash deploy/connector/deploy-stack.sh wireguard`,
        `MODE=wireguard bash deploy/connector/smoke-connector-piloto.sh`,
      )
      break
    default:
      lines.push(
        `# Definir stack no site, depois ver deploy/connector/ e conectividade-vpn-clientes.md`,
      )
  }
  return lines
}

export function tipoRisk(tipo: SiteTipo): 'ok' | 'warn' | 'bad' {
  if (tipo === 'vpn_user_exception') return 'bad'
  if (tipo === 'a_confirmar') return 'warn'
  return 'ok'
}
