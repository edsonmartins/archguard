// CP-5 — connector checklist + admin-first agent control (no day-2 SSH)

import type { JsonObject } from '@/lib/json'
import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import { lookup } from 'node:dns/promises'
import { randomUUID } from 'node:crypto'
import { getDb } from './db'
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
  agentPlanUpgradeForSite,
  agentStageUpgradeForSite,
  agentApplyUpgradeForSite,
  agentRollbackUpgradeForSite,
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
  .handler(async ({ data }): Promise<{
    ok: true
    connector_id: string
    started: boolean
    start: JsonObject | null
    runtime: Awaited<ReturnType<typeof agentListConnectors>>
  }> => {
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
    let startResult: JsonObject | null = null
    if (data.start) {
      startResult = (await agentStart(
        data.connector_id,
        data.stack,
      )) as JsonObject
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
      ok: true as const,
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
    const result = (await agentStop(
      data.connector_id,
      data.stack,
    )) as JsonObject
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

export const planConnectorUpgradeFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({
      slug: z.string().min(1),
      version: z.string().min(1).max(64),
      url: z.string().url().startsWith('https://'),
      sha256: z.string().regex(/^[0-9a-fA-F]{64}$/),
    }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:update', 'gateways:manage'], 'sites:update')
    const site = await getSite(data.slug)
    if (!site) throw new Error('Site não encontrado')
    assertSiteTenantAccess(site, s)
    const plan = await agentPlanUpgradeForSite(data.slug, { version: data.version, url: data.url, sha256: data.sha256 })
    const planData = plan as { upgrade?: { action?: string } }
    const planId = randomUUID()
    const status = planData.upgrade?.action === 'noop' ? 'noop' : 'pending_approval'
    getDb().prepare(
      `INSERT INTO connector_upgrade_plans
         (id, site_slug, version, artifact_url, sha256, status, created_at, created_by)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
    ).run(planId, data.slug, data.version, data.url, data.sha256.toLowerCase(), status, new Date().toISOString(), sessionActor(s))
    recordActivity('POST', `/archgate/connector/${data.slug}/upgrade-plan`, sessionActor(s), 'success', undefined, {
      plan_id: planId,
      version: data.version,
      url: data.url,
      sha256_suffix: data.sha256.slice(-12),
    })
    return { ok: true, plan_id: planId, status, plan }
  })

export const getConnectorUpgradePlansFn = createServerFn({ method: 'GET' })
  .inputValidator((data: unknown) => {
    const r = z.object({ slug: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:read', 'sites:update', 'gateways:manage'], 'sites:read')
    const site = await getSite(data.slug)
    if (!site) throw new Error('Site não encontrado')
    assertSiteTenantAccess(site, s)
    const plans = getDb().prepare(
      `SELECT id, site_slug, version, artifact_url, sha256, status, created_at, created_by, decided_at, decided_by
         FROM connector_upgrade_plans
        WHERE site_slug = ?
        ORDER BY created_at DESC
        LIMIT 25`,
    ).all(data.slug) as Array<{
      id: string
      site_slug: string
      version: string
      artifact_url: string
      sha256: string
      status: string
      created_at: string
      created_by: string
      decided_at?: string | null
      decided_by?: string | null
    }>
    return { plans }
  })

export const decideConnectorUpgradePlanFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({
      slug: z.string().min(1),
      plan_id: z.string().uuid(),
      decision: z.enum(['approve', 'reject']),
    }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:update', 'gateways:manage'], 'sites:update')
    const site = await getSite(data.slug)
    if (!site) throw new Error('Site não encontrado')
    assertSiteTenantAccess(site, s)
    const db = getDb()
    const current = db.prepare(
      'SELECT status FROM connector_upgrade_plans WHERE id = ? AND site_slug = ?',
    ).get(data.plan_id, data.slug) as { status?: string } | undefined
    if (!current) throw new Error('Plano não encontrado')
    if (current.status !== 'pending_approval') {
      throw new Error(`Plano não está pendente (status: ${current.status})`)
    }
    const status = data.decision === 'approve' ? 'approved' : 'rejected'
    const actor = sessionActor(s)
    const decidedAt = new Date().toISOString()
    db.prepare(
      'UPDATE connector_upgrade_plans SET status = ?, decided_at = ?, decided_by = ? WHERE id = ? AND site_slug = ?',
    ).run(status, decidedAt, actor, data.plan_id, data.slug)
    recordActivity('POST', `/archgate/connector/${data.slug}/upgrade-plan/${data.plan_id}/${data.decision}`, actor, 'success', undefined, {
      plan_id: data.plan_id,
      decision: data.decision,
    })
    return { ok: true, plan_id: data.plan_id, status, decided_at: decidedAt, decided_by: actor }
  })

export const rolloutConnectorUpgradeFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({
      slug: z.string().min(1),
      plan_id: z.string().uuid(),
      action: z.enum(['stage', 'apply', 'rollback']),
    }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:update', 'gateways:manage'], 'sites:update')
    const site = await getSite(data.slug)
    if (!site) throw new Error('Site não encontrado')
    assertSiteTenantAccess(site, s)
    const db = getDb()
    const plan = db.prepare(
      'SELECT id, version, artifact_url, sha256, status FROM connector_upgrade_plans WHERE id = ? AND site_slug = ?',
    ).get(data.plan_id, data.slug) as { id: string; version: string; artifact_url: string; sha256: string; status: string } | undefined
    if (!plan) throw new Error('Plano não encontrado')
    if (data.action === 'rollback' && plan.status !== 'applied') {
      throw new Error(`Plano não está aplicado (status: ${plan.status})`)
    }
    if (data.action !== 'rollback' && !['approved', 'staged'].includes(plan.status)) {
      throw new Error(`Plano precisa estar aprovado (status: ${plan.status})`)
    }
    let result: unknown
    let status = plan.status
    if (data.action === 'stage') {
      result = await agentStageUpgradeForSite(data.slug, { version: plan.version, url: plan.artifact_url, sha256: plan.sha256 })
      status = 'staged'
    } else if (data.action === 'apply') {
      if (plan.status !== 'staged') throw new Error(`Plano precisa estar staged (status: ${plan.status})`)
      result = await agentApplyUpgradeForSite(data.slug, plan.version)
      status = 'applied'
    } else {
      result = await agentRollbackUpgradeForSite(data.slug)
      status = 'rolled_back'
    }
    db.prepare('UPDATE connector_upgrade_plans SET status = ?, decided_at = COALESCE(decided_at, ?), decided_by = COALESCE(decided_by, ?) WHERE id = ? AND site_slug = ?')
      .run(status, new Date().toISOString(), sessionActor(s), data.plan_id, data.slug)
    recordActivity('POST', `/archgate/connector/${data.slug}/upgrade-plan/${data.plan_id}/${data.action}`, sessionActor(s), 'success', undefined, {
      plan_id: data.plan_id,
      action: data.action,
    })
    return { ok: true, plan_id: data.plan_id, status, result }
  })

export const createConnectorUpgradeRolloutFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({
      targets: z.array(z.object({ slug: z.string().min(1), plan_id: z.string().uuid() })).min(1).max(100),
      batch_size: z.number().int().min(1).max(10).default(1),
    }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:update', 'gateways:manage'], 'sites:update')
    const db = getDb()
    const rows: Array<{ slug: string; plan_id: string; version: string; artifact_url: string; sha256: string }> = []
    for (const target of data.targets) {
      const site = await getSite(target.slug)
      if (!site) throw new Error(`Site não encontrado: ${target.slug}`)
      assertSiteTenantAccess(site, s)
      const plan = db.prepare(
        'SELECT version, artifact_url, sha256, status FROM connector_upgrade_plans WHERE id = ? AND site_slug = ?',
      ).get(target.plan_id, target.slug) as { version: string; artifact_url: string; sha256: string; status: string } | undefined
      if (!plan) throw new Error(`Plano não encontrado: ${target.slug}`)
      if (plan.status !== 'approved') throw new Error(`Plano ${target.slug} não está aprovado (status: ${plan.status})`)
      rows.push({ slug: target.slug, plan_id: target.plan_id, version: plan.version, artifact_url: plan.artifact_url, sha256: plan.sha256 })
    }
    const first = rows[0]
    if (rows.some((row) => row.version !== first.version || row.artifact_url !== first.artifact_url || row.sha256 !== first.sha256)) {
      throw new Error('Todos os planos da onda precisam ter versão, artefato e SHA-256 idênticos')
    }
    const rolloutId = randomUUID()
    const now = new Date().toISOString()
    const insertRollout = db.prepare(
      `INSERT INTO connector_upgrade_rollouts
        (id, version, artifact_url, sha256, status, batch_size, created_at, created_by, updated_at)
       VALUES (?, ?, ?, ?, 'planned', ?, ?, ?, ?)`,
    )
    const insertTarget = db.prepare(
      `INSERT INTO connector_upgrade_rollout_targets
        (id, rollout_id, site_slug, plan_id, position, status)
       VALUES (?, ?, ?, ?, ?, 'pending')`,
    )
    db.transaction(() => {
      insertRollout.run(rolloutId, first.version, first.artifact_url, first.sha256, data.batch_size, now, sessionActor(s), now)
      rows.forEach((row, index) => insertTarget.run(randomUUID(), rolloutId, row.slug, row.plan_id, index))
    })()
    recordActivity('POST', `/archgate/connector/upgrade-rollouts/${rolloutId}`, sessionActor(s), 'success', undefined, {
      rollout_id: rolloutId,
      targets: rows.map((row) => row.slug),
      batch_size: data.batch_size,
    })
    return { ok: true, rollout_id: rolloutId, status: 'planned', target_count: rows.length, batch_size: data.batch_size }
  })

export const getConnectorUpgradeRolloutFn = createServerFn({ method: 'GET' })
  .inputValidator((data: unknown) => {
    const r = z.object({ rollout_id: z.string().uuid() }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:read', 'sites:update', 'gateways:manage'], 'sites:read')
    const db = getDb()
    const rollout = db.prepare('SELECT * FROM connector_upgrade_rollouts WHERE id = ?').get(data.rollout_id) as Record<string, unknown> | undefined
    if (!rollout) throw new Error('Onda não encontrada')
    const targets = db.prepare('SELECT * FROM connector_upgrade_rollout_targets WHERE rollout_id = ? ORDER BY position').all(data.rollout_id) as Array<Record<string, unknown>>
    for (const target of targets) {
      const site = await getSite(String(target.site_slug))
      if (!site) throw new Error(`Site não encontrado: ${target.site_slug}`)
      assertSiteTenantAccess(site, s)
    }
    return { rollout, targets }
  })

export const advanceConnectorUpgradeRolloutFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({ rollout_id: z.string().uuid() }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:update', 'gateways:manage'], 'sites:update')
    const db = getDb()
    const rollout = db.prepare('SELECT batch_size, status FROM connector_upgrade_rollouts WHERE id = ?').get(data.rollout_id) as { batch_size: number; status: string } | undefined
    if (!rollout) throw new Error('Onda não encontrada')
    if (!['planned', 'running'].includes(rollout.status)) throw new Error(`Onda não pode avançar (status: ${rollout.status})`)
    const targets = db.prepare(
      'SELECT id, site_slug, plan_id FROM connector_upgrade_rollout_targets WHERE rollout_id = ? AND status = ? ORDER BY position LIMIT ?',
    ).all(data.rollout_id, 'pending', rollout.batch_size) as Array<{ id: string; site_slug: string; plan_id: string }>
    if (!targets.length) {
      db.prepare('UPDATE connector_upgrade_rollouts SET status = \'completed\', updated_at = ? WHERE id = ?').run(new Date().toISOString(), data.rollout_id)
      return { ok: true, status: 'completed', processed: 0 }
    }
    db.prepare('UPDATE connector_upgrade_rollouts SET status = \'running\', updated_at = ? WHERE id = ?').run(new Date().toISOString(), data.rollout_id)
    let processed = 0
    for (const target of targets) {
      const started = new Date().toISOString()
      db.prepare('UPDATE connector_upgrade_rollout_targets SET status = \'running\', started_at = ?, error = NULL WHERE id = ?').run(started, target.id)
      try {
        const site = await getSite(target.site_slug)
        if (!site) throw new Error('Site não encontrado')
        assertSiteTenantAccess(site, s)
        const plan = db.prepare('SELECT version, artifact_url, sha256 FROM connector_upgrade_plans WHERE id = ? AND site_slug = ?').get(target.plan_id, target.site_slug) as { version: string; artifact_url: string; sha256: string } | undefined
        if (!plan) throw new Error('Plano não encontrado')
        await agentStageUpgradeForSite(target.site_slug, plan)
        db.prepare('UPDATE connector_upgrade_plans SET status = \'staged\' WHERE id = ?').run(target.plan_id)
        await agentApplyUpgradeForSite(target.site_slug, plan.version)
        db.prepare('UPDATE connector_upgrade_plans SET status = \'applied\' WHERE id = ?').run(target.plan_id)
        db.prepare('UPDATE connector_upgrade_rollout_targets SET status = \'applied\', finished_at = ? WHERE id = ?').run(new Date().toISOString(), target.id)
        processed += 1
      } catch (error) {
        const message = (error as Error).message.slice(0, 500)
        db.prepare('UPDATE connector_upgrade_rollout_targets SET status = \'failed\', error = ?, finished_at = ? WHERE id = ?').run(message, new Date().toISOString(), target.id)
        db.prepare('UPDATE connector_upgrade_rollouts SET status = \'failed\', updated_at = ? WHERE id = ?').run(new Date().toISOString(), data.rollout_id)
        return { ok: false, status: 'failed', processed, failed_site: target.site_slug, error: message }
      }
    }
    const remaining = db.prepare('SELECT COUNT(*) AS count FROM connector_upgrade_rollout_targets WHERE rollout_id = ? AND status = \'pending\'').get(data.rollout_id) as { count: number }
    const status = remaining.count === 0 ? 'completed' : 'running'
    db.prepare('UPDATE connector_upgrade_rollouts SET status = ?, updated_at = ? WHERE id = ?').run(status, new Date().toISOString(), data.rollout_id)
    return { ok: true, status, processed, remaining: remaining.count }
  })

export const rollbackConnectorUpgradeRolloutFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({ rollout_id: z.string().uuid() }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:update', 'gateways:manage'], 'sites:update')
    const db = getDb()
    const targets = db.prepare('SELECT id, site_slug, plan_id FROM connector_upgrade_rollout_targets WHERE rollout_id = ? AND status = \'applied\' ORDER BY position DESC').all(data.rollout_id) as Array<{ id: string; site_slug: string; plan_id: string }>
    let rolledBack = 0
    for (const target of targets) {
      const site = await getSite(target.site_slug)
      if (!site) throw new Error(`Site não encontrado: ${target.site_slug}`)
      assertSiteTenantAccess(site, s)
      await agentRollbackUpgradeForSite(target.site_slug)
      db.prepare('UPDATE connector_upgrade_rollout_targets SET status = \'rolled_back\', finished_at = ? WHERE id = ?').run(new Date().toISOString(), target.id)
      db.prepare('UPDATE connector_upgrade_plans SET status = \'rolled_back\' WHERE id = ?').run(target.plan_id)
      rolledBack += 1
    }
    db.prepare('UPDATE connector_upgrade_rollouts SET status = \'rolled_back\', updated_at = ? WHERE id = ?').run(new Date().toISOString(), data.rollout_id)
    return { ok: true, status: 'rolled_back', rolled_back: rolledBack }
  })
