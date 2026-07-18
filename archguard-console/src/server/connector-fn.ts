// CP-5 — connector checklist + optional lab probes

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
import { getConnectorChecklist, isLabSite } from '@/lib/api/types/site'
import { recordActivity } from './activity-log'
import {
  assertSiteTenantAccess,
  requireAnyPerm,
  requireSession,
  sessionActor,
} from './session-guard'

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
      },
      progress: checklistProgress(items),
      items,
      hints,
      risk,
      probes,
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
