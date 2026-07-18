// CP-5 — connector checklist + admin-first agent control (no day-2 SSH)

import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import { lookup } from 'node:dns/promises'
import { getSite, upsertSite } from './sites'
import {
  checklistProgress,
  deployHints,
  evaluateChecklist,
  tipoRisk,
  type ChecklistEval,
} from '@/lib/connector/checklist'
import {
  getConnectorChecklist,
  isLabSite,
  normalizeSiteConnectors,
} from '@/lib/api/types/site'
import { recordActivity } from './activity-log'
import {
  assertSiteTenantAccess,
  requireAnyPerm,
  requireSession,
  sessionActor,
} from './session-guard'
import {
  agentHealth,
  agentListConnectors,
  agentProbe,
  agentPutConfig,
  agentStart,
  agentStop,
  buildFortiConf,
  connectorAgentConfigured,
  connectorAgentUrl,
} from './connector-agent-proxy'

export type ConnectorProbe = {
  name: string
  ok: boolean
  detail: string
}

/** Best-effort probes from console container (Swarm DNS / TCP). */
async function probeLabConnector(siteSlug: string): Promise<ConnectorProbe[]> {
  const probes: ConnectorProbe[] = []
  const candidates = [
    'archgate-site-piloto_connector',
    'tasks.archgate-site-piloto_connector',
    'archgate-site-piloto_agent',
    'archgate-site-piloto_ovpn-srv',
  ]

  for (const name of candidates) {
    try {
      const r = await lookup(name)
      probes.push({
        name: `dns:${name}`,
        ok: true,
        detail: r.address,
      })
    } catch {
      probes.push({
        name: `dns:${name}`,
        ok: false,
        detail: 'não resolve (normal se piloto não estiver deployado)',
      })
    }
  }

  const host =
    process.env.CONNECTOR_PROBE_HOST ||
    probes.find((p) => p.ok && p.name.includes('connector'))?.detail
  const port = Number(process.env.CONNECTOR_PROBE_PORT || 2223)
  if (host) {
    try {
      const net = await import('node:net')
      const ok = await new Promise<boolean>((resolve) => {
        const s = net.createConnection({ host, port, timeout: 2000 }, () => {
          s.destroy()
          resolve(true)
        })
        s.on('error', () => resolve(false))
        s.on('timeout', () => {
          s.destroy()
          resolve(false)
        })
      })
      probes.push({
        name: `tcp:${host}:${port}`,
        ok,
        detail: ok ? 'open' : 'closed/timeout',
      })
    } catch (e) {
      probes.push({
        name: `tcp:${host}:${port}`,
        ok: false,
        detail: (e as Error).message,
      })
    }
  }

  probes.push({
    name: `site:${siteSlug}`,
    ok: true,
    detail: 'checklist calculado; probe lab é best-effort',
  })

  return probes
}

export const getConnectorStatusFn = createServerFn({ method: 'GET' })
  .inputValidator((data: unknown) => {
    const r = z.object({ slug: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:read', 'sites:update'], 'sites:read')
    const site = await getSite(data.slug)
    if (!site) throw new Error('Site não encontrado')
    assertSiteTenantAccess(site, s)

    let items: ChecklistEval[] = evaluateChecklist(site)
    const hints = deployHints(site)
    const risk = tipoRisk(site.tipo)

    let probes: ConnectorProbe[] = []
    if (isLabSite(site)) {
      probes = await probeLabConnector(site.slug)
      if (probes.some((p) => p.ok && p.name.includes('connector'))) {
        items = items.map((it) =>
          it.id === 'unit_systemd'
            ? {
                ...it,
                done: true,
                source: 'probe',
                detail: 'serviço piloto resolve no Swarm',
              }
            : it,
        )
      }
    }

    let runtime: {
      agent_configured: boolean
      agent_url?: string
      agent_ok?: boolean
      connectors?: Awaited<ReturnType<typeof agentListConnectors>>
      agent_error?: string
    } = {
      agent_configured: connectorAgentConfigured(),
      agent_url: connectorAgentConfigured()
        ? connectorAgentUrl()
        : undefined,
    }
    if (connectorAgentConfigured()) {
      try {
        await agentHealth()
        runtime.agent_ok = true
        runtime.connectors = await agentListConnectors()
      } catch (e) {
        runtime.agent_ok = false
        runtime.agent_error = (e as Error).message
      }
    }

    return {
      site: {
        slug: site.slug,
        cliente: site.cliente,
        stack: site.stack,
        tipo: site.tipo,
        connector_id: site.connector_id,
        ambiente: site.ambiente,
        connector_deployed: site.connector_deployed,
        smoke_operador: site.smoke_operador,
        connectors: normalizeSiteConnectors(site),
      },
      progress: checklistProgress(items),
      items,
      hints,
      risk,
      probes,
      runtime,
      admin_first: true,
    }
  })

export const updateConnectorChecklistFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        slug: z.string().min(1),
        item_id: z.string().min(1),
        done: z.boolean(),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:update'], 'sites:update')
    const site = await getSite(data.slug)
    if (!site) throw new Error('Site não encontrado')
    assertSiteTenantAccess(site, s)

    const prev = getConnectorChecklist(site.stack_meta)
    const next = { ...prev, [data.item_id]: data.done }

    const stack_meta = {
      ...site.stack_meta,
      connector_checklist: next,
    }

    let connector_deployed = site.connector_deployed
    let smoke_operador = site.smoke_operador
    let inventariado = site.inventariado
    if (data.item_id === 'connector_deployed_flag') {
      connector_deployed = data.done
    }
    if (data.item_id === 'smoke_operador') {
      smoke_operador = data.done
    }
    if (data.item_id === 'warpgate_synced' && data.done) {
      inventariado = true
    }

    const actor = sessionActor(s)
    const updated = await upsertSite(
      {
        ...site,
        stack_meta,
        connector_deployed,
        smoke_operador,
        inventariado,
      },
      actor,
    )
    recordActivity(
      'PUT',
      `/archgate/connector/${data.slug}/checklist`,
      actor,
      'success',
      undefined,
      { item_id: data.item_id, done: data.done },
    )

    return {
      items: evaluateChecklist(updated),
      progress: checklistProgress(evaluateChecklist(updated)),
    }
  })

/**
 * Materialize connector config on host via agent (admin-first).
 * Secret is one-shot in the request — never written to site SoT.
 */
export const deployConnectorFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        slug: z.string().min(1),
        connector_id: z.string().min(1).max(64),
        stack: z.enum(['openfortivpn', 'openvpn']),
        /** Full conf body (ovpn file or openfortivpn conf). Preferred for openvpn. */
        config: z.string().max(256_000).optional(),
        /** Structured Forti fields if config omitted */
        forti: z
          .object({
            host: z.string().min(1),
            port: z.number().int().positive().optional(),
            username: z.string().min(1),
            password: z.string().min(1),
            trusted_cert: z.string().optional(),
          })
          .optional(),
        start: z.boolean().default(true),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:update', 'gateways:manage'], 'sites:update')
    const site = await getSite(data.slug)
    if (!site) throw new Error('Site não encontrado')
    assertSiteTenantAccess(site, s)

    if (!connectorAgentConfigured()) {
      throw new Error(
        'Agent não configurado. Bootstrap one-shot: scripts/66-install-connector-agent.sh + rewire console.',
      )
    }

    let conf = data.config?.trim() || ''
    if (!conf && data.stack === 'openfortivpn' && data.forti) {
      conf = buildFortiConf(data.forti)
    }
    if (!conf) {
      throw new Error('Informe config (texto) ou campos Forti (host/user/password)')
    }

    await agentPutConfig(data.connector_id, data.stack, conf)
    let startResult: unknown = null
    if (data.start) {
      startResult = await agentStart(data.connector_id, data.stack)
    }

    const actor = sessionActor(s)
    const connectors = normalizeSiteConnectors(site).map((c) =>
      c.id === data.connector_id
        ? { ...c, stack: data.stack as typeof c.stack }
        : c,
    )
    // Ensure connector id exists on site inventory
    if (!connectors.some((c) => c.id === data.connector_id)) {
      connectors.push({
        id: data.connector_id,
        stack: data.stack,
        tipo: site.tipo,
        subnets: [],
        meta: {},
      })
    }

    await upsertSite(
      {
        ...site,
        connectors,
        connector_deployed: true,
        stack_meta: {
          ...site.stack_meta,
          connector_checklist: {
            ...getConnectorChecklist(site.stack_meta),
            unit_systemd: true,
            connector_deployed_flag: true,
          },
        },
      },
      actor,
    )

    recordActivity(
      'POST',
      `/archgate/connector/${data.slug}/deploy`,
      actor,
      'success',
      undefined,
      {
        connector_id: data.connector_id,
        stack: data.stack,
        started: data.start,
      },
    )

    return {
      ok: true,
      connector_id: data.connector_id,
      started: data.start,
      start: startResult,
      runtime: await agentListConnectors(),
    }
  })

export const stopConnectorFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        slug: z.string().min(1),
        connector_id: z.string().min(1),
        stack: z.enum(['openfortivpn', 'openvpn']),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:update', 'gateways:manage'], 'sites:update')
    const site = await getSite(data.slug)
    if (!site) throw new Error('Site não encontrado')
    assertSiteTenantAccess(site, s)
    const result = await agentStop(data.connector_id, data.stack)
    recordActivity(
      'POST',
      `/archgate/connector/${data.slug}/stop`,
      sessionActor(s),
      'success',
      undefined,
      { connector_id: data.connector_id },
    )
    return { ok: true, result }
  })

export const probeConnectorFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        slug: z.string().min(1),
        host: z.string().min(1),
        port: z.number().int().positive(),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:read', 'sites:update'], 'sites:read')
    const site = await getSite(data.slug)
    if (!site) throw new Error('Site não encontrado')
    assertSiteTenantAccess(site, s)
    return agentProbe(data.host, data.port)
  })
