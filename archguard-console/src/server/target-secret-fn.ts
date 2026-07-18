// W-C3 — Store target password in OpenBao + attach secret_ref on site SoT

import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import { getSite, upsertSite } from './sites'
import { writeTargetSecret, openbaoTokenConfigured } from './openbao-proxy'
import {
  assertSiteTenantAccess,
  requireAnyPerm,
  requireSession,
  sessionActor,
} from './session-guard'
import { recordActivity } from './activity-log'
import { applySiteTargetsFn } from './warpgate-fn'

/**
 * One-shot: password → OpenBao KV; site.targets[].secret_ref updated (no password in SQLite).
 * Optional apply to gateways immediately.
 */
export const storeTargetSecretFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        slug: z.string().min(1),
        target_name: z.string().min(1).max(128),
        password: z.string().min(1).max(2048),
        username: z.string().max(128).optional(),
        /** If target missing, create stub (host required) */
        host: z.string().optional(),
        port: z.number().int().positive().optional(),
        protocolo: z.string().optional(),
        engine: z.enum(['warpgate', 'guacamole']).optional(),
        apply: z.boolean().default(true),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(
      s,
      ['sites:update', 'secrets:manage', 'gateways:manage', 'system:admin'],
      'sites:update',
    )
    if (!openbaoTokenConfigured()) {
      throw new Error(
        'OpenBao sem token no console (OPENBAO_APP_TOKEN). Configure wire 62.',
      )
    }

    const site = await getSite(data.slug)
    if (!site) throw new Error('Site não encontrado')
    assertSiteTenantAccess(site, s)

    const written = await writeTargetSecret({
      name: data.target_name,
      password: data.password,
      username: data.username,
    })

    let targets = [...(site.targets || [])]
    const idx = targets.findIndex((t) => t.nome === data.target_name)
    if (idx >= 0) {
      targets[idx] = {
        ...targets[idx]!,
        secret_ref: written.secret_ref,
        ...(data.username ? { username: data.username } : {}),
      }
    } else {
      if (!data.host) {
        throw new Error(
          'Target não existe no site — informe host para criar stub ou edite o site primeiro',
        )
      }
      targets.push({
        nome: data.target_name,
        engine: data.engine || 'warpgate',
        protocolo: data.protocolo || 'ssh',
        host: data.host,
        port: data.port || 22,
        roles: site.warpgate_roles?.length
          ? site.warpgate_roles
          : [`tenant-${site.slug.replace(/_/g, '-')}`],
        username: data.username,
        secret_ref: written.secret_ref,
      })
    }

    const actor = sessionActor(s)
    const updated = await upsertSite(
      {
        ...site,
        targets,
        inventariado: true,
      },
      actor,
    )

    recordActivity(
      'POST',
      `/archgate/sites/${data.slug}/targets/${data.target_name}/secret`,
      actor,
      'success',
      undefined,
      { secret_ref: written.secret_ref },
    )

    let apply: {
      allOk?: boolean
      results?: Array<{ name: string; ok: boolean; error?: string }>
    } | null = null
    if (data.apply) {
      try {
        apply = await applySiteTargetsFn({
          data: { slug: data.slug },
        })
      } catch (e) {
        apply = {
          allOk: false,
          results: [
            {
              name: data.target_name,
              ok: false,
              error: (e as Error).message,
            },
          ],
        }
      }
    }

    return {
      ok: true,
      secret_ref: written.secret_ref,
      site: updated,
      apply,
      message: `Secret gravado em OpenBao (${written.secret_ref})${
        data.apply
          ? apply?.allOk
            ? ' e apply OK'
            : ' — apply com avisos'
          : ''
      }`,
    }
  })
