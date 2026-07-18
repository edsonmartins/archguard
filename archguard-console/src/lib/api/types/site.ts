// ArchGate site / client configuration (admin console)

export type SiteTipo =
  | 'connector_peer'
  | 'tunnel_agent'
  | 'vpn_user_exception'
  | 'a_confirmar'

export type SiteStack =
  | 'wireguard'
  | 'openvpn'
  | 'openfortivpn'
  | 'cloudflare_tunnel'
  | 'mixed'
  | 'a_confirmar'
  | 'lab_overlay'

export type SiteAmbiente = 'producao' | 'preprod' | 'dr' | 'lab' | 'staging'

export interface SiteTarget {
  nome: string
  engine: 'warpgate' | 'guacamole'
  protocolo: string
  host: string
  port: number
  roles: string[]
  /** Optional gateway login user (not the operator). */
  username?: string
  /**
   * OpenBao KV path for target password/key (no secret value in SoT).
   * Examples: `secret/data/archgate/targets/rio-api-ssh` or `kv/data/targets/x`.
   */
  secret_ref?: string
  /** Optional connector id this target is reached through (multi-VPN sites). */
  connector_id?: string
  notas?: string
}

/** One institutional connectivity path (Forti, OpenVPN, WG, …). */
export interface SiteConnector {
  id: string
  stack: SiteStack
  tipo?: SiteTipo
  /** CIDRs reachable via this connector */
  subnets?: string[]
  /** Non-secret meta (host, forti_host, ovpn_path, secret_ref, mfa, checklist) */
  meta?: SiteStackMeta
  notas?: string
}

/**
 * Non-secret stack metadata. Known keys are typed; extras allowed for forward
 * compatibility (no secrets — use OpenBao / gateway vaults).
 */
export type SiteStackMeta = {
  host?: string
  endpoint?: string
  forti_host?: string
  tunnel_name?: string
  profile_ref?: string
  secret_ref?: string
  ovpn_path?: string
  mfa?: boolean
  warpgate_synced?: boolean
  connector_checklist?: Record<string, boolean>
  mentors_axis_id?: string
  mentors_axis_code?: string
  mentors_axis_status?: string
  cnpj?: string
  on_premise_id?: string
  default_group_id?: string
  /** Escape hatch for stack-specific non-secret flags */
  [key: string]:
    | string
    | boolean
    | number
    | Record<string, boolean>
    | undefined
}

export interface Site {
  slug: string
  cliente: string
  tenant_group: string
  ambiente: SiteAmbiente
  /** Primary / legacy tipo (mirrors connectors[0] when multi). */
  tipo: SiteTipo
  /** Primary / legacy stack; use `mixed` when connectors have >1 stack. */
  stack: SiteStack
  /** Primary / legacy connector id (mirrors connectors[0].id). */
  connector_id: string
  /** Primary / legacy subnets (union or first connector). */
  subnets: string[]
  /** Non-secret stack metadata (hostnames, profile refs, MFA flags, checklist) */
  stack_meta: SiteStackMeta
  /**
   * Multi-VPN / multi-peer paths (Rio: Forti + OpenVPN).
   * Empty/undefined → derived from legacy stack + connector_id on read.
   */
  connectors: SiteConnector[]
  targets: SiteTarget[]
  warpgate_roles: string[]
  notas: string
  inventariado: boolean
  connector_deployed: boolean
  smoke_operador: boolean
  updated_at: string
  updated_by: string | null
}

export type SiteInput = Omit<Site, 'updated_at' | 'updated_by'> & {
  updated_by?: string | null
}

export const SITE_TIPOS: SiteTipo[] = [
  'a_confirmar',
  'connector_peer',
  'tunnel_agent',
  'vpn_user_exception',
]

export const SITE_STACKS: SiteStack[] = [
  'a_confirmar',
  'wireguard',
  'openvpn',
  'openfortivpn',
  'cloudflare_tunnel',
  'mixed',
  'lab_overlay',
]

export const SITE_AMBIENTES: SiteAmbiente[] = [
  'producao',
  'preprod',
  'staging',
  'lab',
  'dr',
]

// --- predicates (pure, safe for client + server) ---

export function isLabSite(site: Pick<Site, 'ambiente' | 'stack' | 'slug'>): boolean {
  return (
    site.ambiente === 'lab' ||
    site.stack === 'lab_overlay' ||
    site.slug.includes('lab') ||
    site.slug.includes('piloto')
  )
}

export function hasVpnEndpointMeta(meta: SiteStackMeta | null | undefined): boolean {
  if (!meta) return false
  return !!(meta.host || meta.endpoint || meta.tunnel_name || meta.forti_host)
}

export function hasSecretRefMeta(meta: SiteStackMeta | null | undefined): boolean {
  if (!meta) return false
  return !!(meta.profile_ref || meta.secret_ref || meta.ovpn_path)
}

export function getConnectorChecklist(
  meta: SiteStackMeta | null | undefined,
): Record<string, boolean> {
  const raw = meta?.connector_checklist
  if (raw && typeof raw === 'object' && !Array.isArray(raw)) {
    return raw
  }
  return {}
}

export function vpnEndpointLabel(meta: SiteStackMeta | null | undefined): string {
  if (!meta) return ''
  return String(meta.host || meta.endpoint || meta.tunnel_name || meta.forti_host || '')
}

/**
 * Normalize connectors: if array empty, synthesize one from legacy site fields.
 * Keeps primary stack/connector_id in sync with connectors[0] when possible.
 */
export function normalizeSiteConnectors(
  site: Pick<
    Site,
    'slug' | 'tipo' | 'stack' | 'connector_id' | 'subnets' | 'stack_meta' | 'connectors'
  >,
): SiteConnector[] {
  const list = site.connectors?.filter((c) => c?.id) ?? []
  if (list.length > 0) {
    return list.map((c) => ({
      id: c.id,
      stack: c.stack || site.stack || 'a_confirmar',
      tipo: c.tipo || site.tipo,
      subnets: c.subnets?.length ? c.subnets : undefined,
      meta: c.meta || {},
      notas: c.notas,
    }))
  }
  const id =
    site.connector_id?.trim() ||
    (site.slug ? `connector-${site.slug}` : 'connector-default')
  return [
    {
      id,
      stack: site.stack || 'a_confirmar',
      tipo: site.tipo || 'a_confirmar',
      subnets: site.subnets?.length ? [...site.subnets] : [],
      meta: { ...(site.stack_meta || {}) },
    },
  ]
}

/** Derive primary stack for sites with multiple connectors. */
export function primaryStackFromConnectors(
  connectors: SiteConnector[],
  fallback: SiteStack = 'a_confirmar',
): SiteStack {
  if (!connectors.length) return fallback
  const stacks = new Set(connectors.map((c) => c.stack).filter(Boolean))
  if (stacks.size > 1) return 'mixed'
  return connectors[0]!.stack || fallback
}
