// W-C1 — Client onboarding wizard (AWS Console–style single flow)

import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import { getSite, upsertSite } from './sites'
import type { SiteConnector, SiteInput, SiteTarget } from '@/lib/api/types/site'
import { primaryStackFromConnectors } from '@/lib/api/types/site'
import { ensureTenantGroup } from './kanidm-admin'
import { recordActivity } from './activity-log'
import {
  assertSiteTenantAccess,
  requireAnyPerm,
  requireSession,
  sessionActor,
} from './session-guard'
import {
  agentProbe,
  agentPutConfig,
  agentStart,
  buildFortiConf,
  connectorAgentConfigured,
} from './connector-agent-proxy'
import { applySiteTargetsFn } from './warpgate-fn'
import { createRole, warpgateConfigured } from './warpgate-proxy'
import { logger } from './logger'

const connectorDraftSchema = z.object({
  id: z.string().min(1).max(64),
  stack: z.enum(['openfortivpn', 'openvpn', 'wireguard', 'lab_overlay', 'a_confirmar']),
  subnets: z.array(z.string()).default([]),
  /** Deploy now via agent (needs secret) */
  materialize: z.boolean().default(false),
  /** Full conf / ovpn when materialize */
  config: z.string().max(256_000).optional(),
  forti: z
    .object({
      host: z.string().min(1),
      port: z.number().int().positive().optional(),
      username: z.string().min(1),
      password: z.string().min(1),
      trusted_cert: z.string().optional(),
    })
    .optional(),
})

const targetDraftSchema = z.object({
  nome: z.string().min(1),
  engine: z.enum(['warpgate', 'guacamole']).default('warpgate'),
  protocolo: z.string().min(1),
  host: z.string().min(1),
  port: z.number().int().positive(),
  roles: z.array(z.string()).default([]),
  username: z.string().optional(),
  secret_ref: z.string().optional(),
  connector_id: z.string().optional(),
  notas: z.string().optional(),
})

const wizardSchema = z.object({
  cliente: z.string().min(1).max(200),
  slug: z.string().min(1).max(64),
  tenant_group: z.string().min(1).max(128).optional(),
  ambiente: z
    .enum(['producao', 'preprod', 'dr', 'lab', 'staging'])
    .default('producao'),
  warpgate_roles: z.array(z.string()).default([]),
  notas: z.string().default(''),
  connectors: z.array(connectorDraftSchema).default([]),
  targets: z.array(targetDraftSchema).default([]),
  /** Apply targets to Warpgate/Guac after save */
  apply_gateways: z.boolean().default(true),
  /** Optional TCP probes host:port */
  probes: z
    .array(
      z.object({
        host: z.string().min(1),
        port: z.number().int().positive(),
      }),
    )
    .default([]),
  /** One-shot passwords for apply (target name → password) */
  target_passwords: z.record(z.string()).optional(),
})

export type WizardStepResult = {
  step: string
  ok: boolean
  detail?: string
}

function normalizeSlug(slug: string): string {
  return slug
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '_')
    .replace(/^_+|_+$/g, '')
}

/**
 * Single-shot onboarding: site SoT + tenant group + connectors agent + gateway apply + probes.
 * Secrets in connector/target payloads are one-shot (not persisted in site JSON except secret_ref).
 */
export const runClientOnboardingWizardFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = wizardSchema.safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:create', 'sites:update'], 'sites:update')
    const actor = sessionActor(s)
    const steps: WizardStepResult[] = []
    const slug = normalizeSlug(data.slug)
    if (!slug) throw new Error('slug inválido')

    const tenant =
      data.tenant_group?.trim() ||
      (slug.startsWith('tenant_') ? slug : `tenant_${slug}`)

    const roleDefault =
      data.warpgate_roles.length > 0
        ? data.warpgate_roles
        : [`tenant-${slug.replace(/_/g, '-')}`]

    const connectors: SiteConnector[] = data.connectors.map((c) => ({
      id: c.id,
      stack: c.stack as SiteConnector['stack'],
      tipo: 'tunnel_agent',
      subnets: c.subnets,
      meta: {},
    }))

    const targets: SiteTarget[] = data.targets.map((t) => ({
      nome: t.nome,
      engine: t.engine,
      protocolo: t.protocolo,
      host: t.host,
      port: t.port,
      roles: t.roles.length ? t.roles : roleDefault,
      username: t.username,
      secret_ref: t.secret_ref,
      connector_id: t.connector_id,
      notas: t.notas,
    }))

    const stack = primaryStackFromConnectors(
      connectors,
      connectors[0]?.stack || 'a_confirmar',
    )

    const siteInput: SiteInput = {
      slug,
      cliente: data.cliente.trim(),
      tenant_group: tenant,
      ambiente: data.ambiente,
      tipo: connectors.length ? 'tunnel_agent' : 'a_confirmar',
      stack,
      connector_id: connectors[0]?.id || `connector-${slug}`,
      subnets: connectors[0]?.subnets || [],
      stack_meta: {},
      connectors,
      targets,
      warpgate_roles: roleDefault,
      notas: data.notas || 'Criado pelo wizard Novo cliente (console)',
      inventariado: targets.length > 0,
      connector_deployed: false,
      smoke_operador: false,
    }

    // Tenant access check for existing site
    const existing = await getSite(slug)
    if (existing) {
      assertSiteTenantAccess(existing, s)
    } else {
      assertSiteTenantAccess(
        {
          ...siteInput,
          updated_at: new Date().toISOString(),
          updated_by: null,
        } as Parameters<typeof assertSiteTenantAccess>[0],
        s,
      )
    }

    // Step 1 — SoT
    let site
    try {
      site = await upsertSite(siteInput, actor)
      steps.push({ step: 'site_save', ok: true, detail: site.slug })
    } catch (e) {
      steps.push({
        step: 'site_save',
        ok: false,
        detail: (e as Error).message,
      })
      throw e
    }

    // Step 2 — Kanidm tenant group
    try {
      const kg = await ensureTenantGroup(tenant, data.cliente)
      steps.push({
        step: 'tenant_group',
        ok: kg.action !== 'error',
        detail: `${tenant}: ${kg.action}${kg.error ? ' — ' + kg.error : ''}`,
      })
    } catch (e) {
      steps.push({
        step: 'tenant_group',
        ok: false,
        detail: (e as Error).message,
      })
    }

    // Step 3 — Warpgate roles (best-effort)
    if (warpgateConfigured()) {
      for (const role of roleDefault) {
        try {
          await createRole(role)
          steps.push({ step: 'warpgate_role', ok: true, detail: role })
        } catch (e) {
          steps.push({
            step: 'warpgate_role',
            ok: false,
            detail: `${role}: ${(e as Error).message}`,
          })
        }
      }
    } else {
      steps.push({
        step: 'warpgate_role',
        ok: true,
        detail: 'skipped (warpgate not configured)',
      })
    }

    // Step 4 — Materialize connectors via agent
    let anyConnectorOk = false
    for (const c of data.connectors) {
      if (!c.materialize) {
        steps.push({
          step: `connector:${c.id}`,
          ok: true,
          detail: 'skipped (materialize=false)',
        })
        continue
      }
      if (!connectorAgentConfigured()) {
        steps.push({
          step: `connector:${c.id}`,
          ok: false,
          detail: 'agent não configurado',
        })
        continue
      }
      if (c.stack !== 'openfortivpn' && c.stack !== 'openvpn') {
        steps.push({
          step: `connector:${c.id}`,
          ok: false,
          detail: `stack ${c.stack} não materializável pelo agent ainda`,
        })
        continue
      }
      try {
        let conf = c.config?.trim() || ''
        if (!conf && c.stack === 'openfortivpn' && c.forti) {
          conf = buildFortiConf(c.forti)
        }
        if (!conf) {
          throw new Error('config ou forti obrigatório para materializar')
        }
        await agentPutConfig(c.id, c.stack, conf)
        await agentStart(c.id, c.stack)
        anyConnectorOk = true
        steps.push({
          step: `connector:${c.id}`,
          ok: true,
          detail: 'materialized+started',
        })
      } catch (e) {
        steps.push({
          step: `connector:${c.id}`,
          ok: false,
          detail: (e as Error).message,
        })
      }
    }

    // Step 5 — Apply gateways
    if (data.apply_gateways && targets.length > 0) {
      try {
        // Call apply logic by reusing applySiteTargetsFn handler path
        const applyRes = await applySiteTargetsFn({
          data: {
            slug,
            passwords: data.target_passwords,
          },
        })
        steps.push({
          step: 'apply_gateways',
          ok: applyRes.allOk,
          detail: applyRes.results
            .map(
              (r) =>
                `${r.ok ? 'OK' : 'FAIL'} ${r.name}${r.error ? ': ' + r.error : ''}`,
            )
            .join('; '),
        })
      } catch (e) {
        steps.push({
          step: 'apply_gateways',
          ok: false,
          detail: (e as Error).message,
        })
      }
    } else {
      steps.push({
        step: 'apply_gateways',
        ok: true,
        detail: targets.length ? 'skipped' : 'no targets',
      })
    }

    // Step 6 — Probes
    let smokeOk = false
    if (data.probes.length && connectorAgentConfigured()) {
      for (const p of data.probes) {
        try {
          const r = await agentProbe(p.host, p.port)
          steps.push({
            step: `probe:${p.host}:${p.port}`,
            ok: r.ok,
            detail: r.detail,
          })
          if (r.ok) smokeOk = true
        } catch (e) {
          steps.push({
            step: `probe:${p.host}:${p.port}`,
            ok: false,
            detail: (e as Error).message,
          })
        }
      }
    } else if (data.probes.length) {
      steps.push({
        step: 'probe',
        ok: false,
        detail: 'agent não configurado',
      })
    }

    // Final flags on site
    try {
      site = await upsertSite(
        {
          ...site,
          connector_deployed: anyConnectorOk || site.connector_deployed,
          smoke_operador: smokeOk || site.smoke_operador,
          inventariado: targets.length > 0,
        },
        actor,
      )
      steps.push({ step: 'finalize', ok: true })
    } catch (e) {
      steps.push({
        step: 'finalize',
        ok: false,
        detail: (e as Error).message,
      })
    }

    const allCriticalOk = steps
      .filter((s) => s.step === 'site_save' || s.step.startsWith('connector:'))
      .every((s) => s.ok || s.detail?.includes('skipped'))

    recordActivity(
      'POST',
      `/archgate/onboarding/wizard/${slug}`,
      actor,
      allCriticalOk ? 'success' : 'error',
      allCriticalOk ? undefined : 'partial failure',
      { steps: steps.map((x) => `${x.step}:${x.ok ? 'ok' : 'fail'}`).join(',') },
    )

    logger.info({ slug, steps: steps.length }, 'onboarding wizard finished')

    return {
      ok: allCriticalOk,
      site,
      steps,
      next: {
        site_url: `/sites/${slug}`,
        gateways_url: '/gateways',
        identities_url: '/identities',
      },
    }
  })
