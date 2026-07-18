// Serialize / parse site fichas with the `yaml` package (no hand-rolled parser).

import { parse as yamlParse, stringify as yamlStringify } from 'yaml'
import type {
  Site,
  SiteConnector,
  SiteInput,
  SiteStackMeta,
  SiteTarget,
} from '@/lib/api/types/site'
import { normalizeSiteConnectors } from '@/lib/api/types/site'

const CONNECTIVITY_RESERVED = new Set([
  'tipo',
  'stack',
  'connector_id',
  'subnets_alvo',
  'subnets',
])

/** Export site as ficha YAML (no secrets). */
export function siteToYaml(site: Site): string {
  const doc = {
    cliente: site.cliente,
    slug: site.slug,
    tenant_group: site.tenant_group,
    ambiente: site.ambiente,
    warpgate_roles: site.warpgate_roles,
    conectividade: {
      tipo: site.tipo,
      stack: site.stack,
      connector_id: site.connector_id,
      subnets_alvo: site.subnets,
      ...site.stack_meta,
    },
    // Multi-VPN (Rio: Forti + OpenVPN). Empty connectors omitted for compact fichas.
    ...(site.connectors?.length
      ? {
          connectors: site.connectors.map((c) => ({
            id: c.id,
            stack: c.stack,
            ...(c.tipo ? { tipo: c.tipo } : {}),
            ...(c.subnets?.length ? { subnets: c.subnets } : {}),
            ...(c.meta && Object.keys(c.meta).length ? { meta: c.meta } : {}),
            ...(c.notas ? { notas: c.notas } : {}),
          })),
        }
      : {}),
    targets: site.targets.map((t) => ({
      nome: t.nome,
      engine: t.engine,
      protocolo: t.protocolo,
      host: t.host,
      port: t.port,
      roles: t.roles,
      ...(t.username ? { username: t.username } : {}),
      ...(t.secret_ref ? { secret_ref: t.secret_ref } : {}),
      ...(t.connector_id ? { connector_id: t.connector_id } : {}),
      ...(t.notas ? { notas: t.notas } : {}),
    })),
    estado: {
      inventariado: site.inventariado,
      connector_deployed: site.connector_deployed,
      smoke_operador: site.smoke_operador,
      data_revisao: site.updated_at.slice(0, 10),
    },
    notas: site.notas || undefined,
    credenciais_alvo: 'warpgate_and_guacamole_only',
    anti_padrao: 'never_document_direct_ip_or_target_password_to_operators',
  }
  const header = [
    `# ArchGate site ficha — export control plane`,
    `# slug: ${site.slug}`,
    `# updated: ${site.updated_at}`,
    `# updated_by: ${site.updated_by || '—'}`,
    ``,
  ].join('\n')
  return (
    header +
    yamlStringify(doc, {
      lineWidth: 100,
      defaultStringType: 'QUOTE_DOUBLE',
      defaultKeyType: 'PLAIN',
    })
  )
}

export function sitesToYamlBundle(sites: Site[]): string {
  const parts = sites.map((s) => siteToYaml(s))
  return (
    `# ArchGate sites bundle — ${sites.length} site(s)\n` +
    `# exported: ${new Date().toISOString()}\n\n` +
    parts.join('\n---\n\n')
  )
}

/** Parse a site ficha (YAML or JSON). Multi-doc: first document only. */
export function parseSiteDocument(text: string): SiteInput {
  const trimmed = text.trim()
  const first = trimmed.split(/\n---\n/)[0]!.trim()

  if (first.startsWith('{')) {
    return normalizeSiteInput(JSON.parse(first) as Record<string, unknown>)
  }

  let raw: unknown
  try {
    raw = yamlParse(first)
  } catch (e) {
    throw new Error(`YAML inválido: ${(e as Error).message}`)
  }

  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
    throw new Error('Documento de site deve ser um objeto YAML/JSON')
  }

  return normalizeSiteInput(raw as Record<string, unknown>)
}

function normalizeSiteInput(raw: Record<string, unknown>): SiteInput {
  const conn = (raw.conectividade || raw.connectivity || {}) as Record<
    string,
    unknown
  >
  const estado = (raw.estado || raw.state || {}) as Record<string, unknown>
  const targetsRaw = (raw.targets || []) as unknown[]

  const stack_meta: SiteStackMeta = {}
  for (const [k, v] of Object.entries(conn)) {
    if (CONNECTIVITY_RESERVED.has(k)) continue
    if (
      typeof v === 'string' ||
      typeof v === 'boolean' ||
      typeof v === 'number'
    ) {
      stack_meta[k] = v
    } else if (typeof v === 'object' && v !== null && !Array.isArray(v)) {
      // only boolean maps (checklist)
      const entries = Object.entries(v as Record<string, unknown>)
      if (entries.every(([, x]) => typeof x === 'boolean')) {
        stack_meta[k] = v as Record<string, boolean>
      }
    }
  }

  const targets: SiteTarget[] = targetsRaw.map((t) => {
    const x = t as Record<string, unknown>
    return {
      nome: String(x.nome || x.name || ''),
      engine: (x.engine as SiteTarget['engine']) || 'warpgate',
      protocolo: String(x.protocolo || x.protocol || 'ssh'),
      host: String(x.host || ''),
      port: Number(x.port || 22),
      roles: Array.isArray(x.roles)
        ? (x.roles as string[])
        : String(x.roles || '')
            .split(',')
            .map((r) => r.trim())
            .filter(Boolean),
      username: x.username ? String(x.username) : undefined,
      secret_ref: x.secret_ref ? String(x.secret_ref) : undefined,
      connector_id: x.connector_id ? String(x.connector_id) : undefined,
      notas: x.notas ? String(x.notas) : undefined,
    }
  })

  const slug = String(raw.slug || '')
  const subnets = (conn.subnets_alvo ||
    conn.subnets ||
    raw.subnets ||
    []) as string[]

  const connectorsRaw = (raw.connectors || raw.conectores || []) as unknown[]
  const connectors: SiteConnector[] = connectorsRaw
    .map((c) => {
      const x = c as Record<string, unknown>
      const meta =
        x.meta && typeof x.meta === 'object' && !Array.isArray(x.meta)
          ? (x.meta as SiteStackMeta)
          : undefined
      return {
        id: String(x.id || x.connector_id || ''),
        stack: (x.stack as SiteConnector['stack']) || 'a_confirmar',
        tipo: x.tipo ? (x.tipo as SiteConnector['tipo']) : undefined,
        subnets: Array.isArray(x.subnets)
          ? (x.subnets as string[]).map(String)
          : undefined,
        meta,
        notas: x.notas ? String(x.notas) : undefined,
      }
    })
    .filter((c) => c.id)

  const draft = {
    slug,
    cliente: String(raw.cliente || raw.client || slug),
    tenant_group: String(
      raw.tenant_group || raw.tenant || (slug ? `tenant_${slug}` : ''),
    ),
    ambiente: (raw.ambiente as SiteInput['ambiente']) || 'producao',
    tipo:
      (conn.tipo as SiteInput['tipo']) ||
      (raw.tipo as SiteInput['tipo']) ||
      'a_confirmar',
    stack:
      (conn.stack as SiteInput['stack']) ||
      (raw.stack as SiteInput['stack']) ||
      'a_confirmar',
    connector_id: String(conn.connector_id || raw.connector_id || ''),
    subnets: Array.isArray(subnets) ? subnets.map(String) : [],
    stack_meta,
    connectors,
    targets,
    warpgate_roles: Array.isArray(raw.warpgate_roles)
      ? (raw.warpgate_roles as string[])
      : [],
    notas: String(raw.notas || ''),
    inventariado: Boolean(estado.inventariado ?? raw.inventariado ?? false),
    connector_deployed: Boolean(
      estado.connector_deployed ?? raw.connector_deployed ?? false,
    ),
    smoke_operador: Boolean(
      estado.smoke_operador ?? raw.smoke_operador ?? false,
    ),
  }

  return {
    ...draft,
    connectors: normalizeSiteConnectors(draft),
  }
}
