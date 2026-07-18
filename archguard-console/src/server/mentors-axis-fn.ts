import { createServerFn } from '@tanstack/react-start'
import { getSite, upsertSite } from './sites'
import type { SiteInput } from '@/lib/api/types/site'
import { recordActivity } from './activity-log'
import {
  axisConnectionInfo,
  listProprietarios,
  mentorsAxisConfigured,
  mentorsAxisMode,
  slugFromProprietario,
  type AxisProprietario,
} from './mentors-axis-proxy'
import {
  requireAnyPerm,
  requireSession,
  sessionActor,
} from './session-guard'
import { ensureTenantGroup, kanidmAdminConfigured } from './kanidm-admin'
import { ensureRole, warpgateConfigured } from './warpgate-proxy'

export type OrchestrationStep = {
  system: 'site' | 'kanidm' | 'warpgate'
  action: string
  detail?: string
  ok: boolean
}

export const getMentorsAxisStatusFn = createServerFn({ method: 'GET' }).handler(
  async () => {
    requireSession()
    const info = axisConnectionInfo()
    return {
      configured: mentorsAxisConfigured(),
      mode: mentorsAxisMode(),
      url: info.url,
      tenant_id: info.tenant_id,
      auth: info.auth,
      endpoints: info.endpoints,
      kanidm_ensure_groups: kanidmAdminConfigured(),
      warpgate_ensure_roles: warpgateConfigured(),
      orchestration: [
        'site_upsert',
        'kanidm_tenant_group',
        'warpgate_tenant_role',
      ],
      source_repo: 'mentors-axis-server-api',
    }
  },
)

export const listMentorsAxisProprietariosFn = createServerFn({
  method: 'GET',
}).handler(async () => {
  const s = requireSession()
  requireAnyPerm(s, ['sites:create', 'sites:update'], 'sites:update')
  const items = await listProprietarios(0, 200)
  const out = []
  for (const p of items) {
    const slug = slugFromProprietario(p)
    out.push({
      id: p.id,
      code: p.code,
      descricao: p.descricao,
      status:
        typeof p.status === 'string'
          ? p.status
          : (p.status as { name?: string } | undefined)?.name || '',
      cnpj: p.cnpj,
      admin_email: p.adminEmail || p.email,
      slug_sugerido: slug,
      tenant_group: `tenant_${slug}`,
      warpgate_role: `tenant-${slug.replace(/_/g, '-')}`,
      site_exists: !!(await getSite(slug)),
    })
  }
  return out
})

async function proprietarioToSiteInput(
  p: AxisProprietario,
): Promise<SiteInput> {
  const slug = slugFromProprietario(p)
  const existing = await getSite(slug)
  const statusStr =
    typeof p.status === 'string'
      ? p.status
      : (p.status as { name?: string } | undefined)?.name || ''
  const status = statusStr.toUpperCase()
  const inactive =
    status.includes('INATIV') ||
    status.includes('BLOQ') ||
    status.includes('CANCEL') ||
    status.includes('INADIMPL')

  const adminEmail = p.adminEmail || p.email || ''
  const axisMeta = {
    mentors_axis_id: p.id,
    mentors_axis_code: p.code || '',
    mentors_axis_status: statusStr,
    ...(p.cnpj ? { cnpj: p.cnpj } : {}),
    ...(p.onPremiseId ? { on_premise_id: p.onPremiseId } : {}),
    ...(p.defaultGroupId ? { default_group_id: p.defaultGroupId } : {}),
    ...(adminEmail ? { admin_email: adminEmail } : {}),
    ...(inactive ? { access_review: true } : {}),
  }

  const warpgate_roles = [`tenant-${slug.replace(/_/g, '-')}`]
  const tenant_group = `tenant_${slug}`

  if (existing) {
    return {
      ...existing,
      cliente: p.descricao || existing.cliente,
      tenant_group: existing.tenant_group || tenant_group,
      warpgate_roles: existing.warpgate_roles?.length
        ? existing.warpgate_roles
        : warpgate_roles,
      notas: [
        existing.notas,
        p.cnpj ? `CNPJ Axis: ${p.cnpj}` : '',
        adminEmail ? `Admin Axis: ${adminEmail}` : '',
        inactive ? `⚠ Status Axis: ${statusStr} — revisar acesso` : '',
      ]
        .filter(Boolean)
        .join('\n')
        .slice(0, 2000),
      stack_meta: {
        ...existing.stack_meta,
        ...axisMeta,
      },
    }
  }

  return {
    slug,
    cliente: p.descricao || slug,
    tenant_group,
    ambiente: 'producao',
    tipo: 'a_confirmar',
    stack: 'a_confirmar',
    connector_id: `connector-${slug}`,
    subnets: [],
    stack_meta: axisMeta,
    targets: [],
    warpgate_roles,
    notas: [
      'Sincronizado do Mentors Axis (CP-7). Configure stack VPN e targets.',
      p.cnpj ? `CNPJ: ${p.cnpj}` : '',
      adminEmail ? `Admin Axis: ${adminEmail}` : '',
      inactive ? `Status Axis: ${statusStr} — revisar acesso` : '',
    ]
      .filter(Boolean)
      .join('\n'),
    inventariado: false,
    connector_deployed: false,
    smoke_operador: false,
  }
}

/**
 * Orchestrate Axis → ArchGate (ADR-004 CP-7 + bootstrap identity/gateway):
 * 1. upsert site ficha
 * 2. ensure Kanidm tenant_* group
 * 3. ensure Warpgate role(s) for site
 * Does NOT create persons (Fase 4 — requires HITL / Axis user directory).
 */
export const syncMentorsAxisTenantsFn = createServerFn({
  method: 'POST',
}).handler(async () => {
  const s = requireSession()
  requireAnyPerm(s, ['sites:create', 'sites:update'], 'sites:update')
  const actor = sessionActor(s)
  const proprietarios = await listProprietarios(0, 200)
  const wgOk = warpgateConfigured()

  const results: {
    slug: string
    cliente: string
    action: 'created' | 'updated' | 'skipped'
    axis_id: string
    tenant_group: string
    admin_email?: string
    kanidm_group?: string
    warpgate_role?: string
    steps: OrchestrationStep[]
  }[] = []

  for (const p of proprietarios) {
    if (!p.id && !p.descricao && !p.code) continue
    const slug = slugFromProprietario(p)
    const existed = !!(await getSite(slug))
    const input = await proprietarioToSiteInput(p)
    const steps: OrchestrationStep[] = []

    try {
      await upsertSite(input, actor)
      steps.push({
        system: 'site',
        action: existed ? 'updated' : 'created',
        detail: input.slug,
        ok: true,
      })
    } catch (e) {
      steps.push({
        system: 'site',
        action: 'error',
        detail: (e as Error).message,
        ok: false,
      })
      results.push({
        slug,
        cliente: input.cliente,
        action: 'skipped',
        axis_id: p.id,
        tenant_group: input.tenant_group,
        steps,
      })
      continue
    }

    // Kanidm tenant group
    const kg = await ensureTenantGroup(input.tenant_group, input.cliente)
    steps.push({
      system: 'kanidm',
      action: kg.action,
      detail: kg.error || input.tenant_group,
      ok: kg.action === 'created' || kg.action === 'exists',
    })

    // Warpgate roles
    let warpgateRoleNote = 'skipped'
    if (wgOk) {
      const roleNotes: string[] = []
      for (const roleName of input.warpgate_roles || []) {
        if (!roleName) continue
        try {
          const role = await ensureRole(
            roleName,
            `ArchGate tenant role for ${input.cliente}`,
          )
          roleNotes.push(`${role.name}:ok`)
          steps.push({
            system: 'warpgate',
            action: 'ensure_role',
            detail: role.name,
            ok: true,
          })
        } catch (e) {
          roleNotes.push(`${roleName}:err`)
          steps.push({
            system: 'warpgate',
            action: 'ensure_role',
            detail: (e as Error).message,
            ok: false,
          })
        }
      }
      warpgateRoleNote = roleNotes.join('; ') || 'none'
    } else {
      steps.push({
        system: 'warpgate',
        action: 'skipped',
        detail: 'WARPGATE_ADMIN_PASSWORD not configured',
        ok: true,
      })
      warpgateRoleNote = 'unconfigured'
    }

    results.push({
      slug,
      cliente: input.cliente,
      action: existed ? 'updated' : 'created',
      axis_id: p.id,
      tenant_group: input.tenant_group,
      admin_email: p.adminEmail || p.email,
      kanidm_group: `${kg.action}${kg.error ? `: ${kg.error}` : ''}`,
      warpgate_role: warpgateRoleNote,
      steps,
    })
  }

  const orchestrationOk = results.every((r) => r.steps.every((st) => st.ok))

  recordActivity(
    'POST',
    '/archgate/integrations/mentors-axis/sync',
    actor,
    orchestrationOk ? 'success' : 'error',
    orchestrationOk ? undefined : 'partial orchestration failures',
    {
      mode: mentorsAxisMode(),
      count: results.length,
      kanidm_groups: results.map((r) => r.kanidm_group),
      warpgate_roles: results.map((r) => r.warpgate_role),
    },
  )

  return {
    mode: mentorsAxisMode(),
    results,
    created: results.filter((r) => r.action === 'created').length,
    updated: results.filter((r) => r.action === 'updated').length,
    kanidm_created: results.filter((r) =>
      r.kanidm_group?.startsWith('created'),
    ).length,
    kanidm_exists: results.filter((r) =>
      r.kanidm_group?.startsWith('exists'),
    ).length,
    warpgate_roles_ok: results.filter((r) =>
      r.steps.some((st) => st.system === 'warpgate' && st.ok && st.action === 'ensure_role'),
    ).length,
    steps_failed: results.reduce(
      (n, r) => n + r.steps.filter((st) => !st.ok).length,
      0,
    ),
    orchestration_ok: orchestrationOk,
  }
})
