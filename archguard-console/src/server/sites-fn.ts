// Client-facing server functions for site inventory (no better-sqlite3 on client).

import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import {
  deleteSite,
  getSite,
  listSites,
  seedDefaultSitesIfEmpty,
  upsertSite,
} from './sites'
import type { Site, SiteInput } from '@/lib/api/types/site'
import { recordActivity } from './activity-log'
import {
  parseSiteDocument,
  siteToYaml,
  sitesToYamlBundle,
} from '@/lib/sites/site-yaml'
import {
  assertSiteTenantAccess,
  filterSitesByTenant,
  requireAnyPerm,
  requireSession,
  sessionActor,
} from './session-guard'
import { ensureTenantGroup } from './kanidm-admin'
import { sitesBackend } from './sites'

const stackMetaSchema = z
  .record(
    z.union([
      z.string(),
      z.boolean(),
      z.number(),
      z.record(z.boolean()),
    ]),
  )
  .default({})

const siteConnectorSchema = z.object({
  id: z.string().min(1).max(128),
  stack: z.string().min(1).max(64),
  tipo: z.string().max(64).optional(),
  subnets: z.array(z.string()).optional(),
  meta: stackMetaSchema.optional(),
  notas: z.string().optional(),
})

const siteInputSchema = z.object({
  slug: z.string().min(1).max(64),
  cliente: z.string().min(1).max(200),
  tenant_group: z.string().min(1).max(128),
  ambiente: z.string().min(1).max(32),
  tipo: z.string().min(1).max(64),
  stack: z.string().min(1).max(64),
  connector_id: z.string().max(128).default(''),
  subnets: z.array(z.string()).default([]),
  stack_meta: stackMetaSchema,
  connectors: z.array(siteConnectorSchema).default([]),
  targets: z
    .array(
      z.object({
        nome: z.string(),
        engine: z.enum(['warpgate', 'guacamole']),
        protocolo: z.string(),
        host: z.string(),
        port: z.number().int().positive(),
        roles: z.array(z.string()),
        username: z.string().optional(),
        secret_ref: z.string().optional(),
        connector_id: z.string().optional(),
        notas: z.string().optional(),
      }),
    )
    .default([]),
  warpgate_roles: z.array(z.string()).default([]),
  notas: z.string().default(''),
  inventariado: z.boolean().default(false),
  connector_deployed: z.boolean().default(false),
  smoke_operador: z.boolean().default(false),
})

export const listSitesFn = createServerFn({ method: 'GET' }).handler(
  async (): Promise<Site[]> => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:read', 'sites:update', 'sites:create'], 'sites:read')
    // Seed is NOT on GET — use seedDefaultSitesFn (POST, admin).
    return filterSitesByTenant(await listSites(), s)
  },
)

export const getSiteFn = createServerFn({ method: 'GET' })
  .inputValidator((data: unknown) => {
    const r = z.object({ slug: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }): Promise<Site | null> => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:read', 'sites:update', 'sites:create'], 'sites:read')
    const site = await getSite(data.slug)
    if (!site) return null
    const allowed = filterSitesByTenant([site], s)
    return allowed[0] ?? null
  })

export const upsertSiteFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = siteInputSchema.safeParse(data)
    if (!r.success) throw new Error(`Invalid site: ${r.error.message}`)
    return r.data as SiteInput
  })
  .handler(async ({ data }): Promise<Site> => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:update', 'sites:create'], 'sites:update')
    // Tenant-scoped writes: cannot create outside derived tenants (admins ok).
    const probe: Site = {
      ...data,
      updated_at: new Date().toISOString(),
      updated_by: null,
    } as Site
    assertSiteTenantAccess(probe, s)
    const actor = sessionActor(s)
    try {
      const site = await upsertSite(data, actor)
      // ADR-004: ensure tenant_* group in Kanidm (best-effort; never blocks write)
      const kg = await ensureTenantGroup(site.tenant_group, site.cliente)
      recordActivity(
        'PUT',
        `/archgate/sites/${site.slug}`,
        actor,
        'success',
        undefined,
        {
          slug: site.slug,
          cliente: site.cliente,
          sites_backend: sitesBackend(),
          kanidm_group: kg.action,
        },
      )
      return site
    } catch (e) {
      recordActivity(
        'PUT',
        `/archgate/sites/${data.slug}`,
        actor,
        'error',
        (e as Error).message,
      )
      throw e
    }
  })

export const deleteSiteFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({ slug: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }): Promise<{ ok: boolean }> => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:update', 'sites:delete'], 'sites:update')
    const existing = await getSite(data.slug)
    if (existing) assertSiteTenantAccess(existing, s)
    const actor = sessionActor(s)
    try {
      const ok = await deleteSite(data.slug)
      recordActivity(
        'DELETE',
        `/archgate/sites/${data.slug}`,
        actor,
        ok ? 'success' : 'error',
        ok ? undefined : 'not found',
      )
      return { ok }
    } catch (e) {
      recordActivity(
        'DELETE',
        `/archgate/sites/${data.slug}`,
        actor,
        'error',
        (e as Error).message,
      )
      throw e
    }
  })

export const exportSiteYamlFn = createServerFn({ method: 'GET' })
  .inputValidator((data: unknown) => {
    const r = z.object({ slug: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:read', 'sites:update', 'sites:create'], 'sites:read')
    const site = await getSite(data.slug)
    if (!site) throw new Error('Site não encontrado')
    assertSiteTenantAccess(site, s)
    return {
      filename: `${site.slug}.yaml`,
      yaml: siteToYaml(site),
    }
  })

export const exportAllSitesYamlFn = createServerFn({ method: 'GET' }).handler(
  async () => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:read', 'sites:update', 'sites:create'], 'sites:read')
    const sites = filterSitesByTenant(await listSites(), s)
    return {
      filename: `archgate-sites-${new Date().toISOString().slice(0, 10)}.yaml`,
      yaml: sitesToYamlBundle(sites),
      count: sites.length,
    }
  },
)

export const importSiteYamlFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        yaml: z.string().min(3).max(500_000),
        /** If true, only validate + return preview without write */
        dry_run: z.boolean().optional(),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, ['sites:update', 'sites:create'], 'sites:update')
    const actor = sessionActor(s)

    const docs = data.yaml
      .split(/\n---\n/)
      .map((d) => d.trim())
      .filter((d) => d && !d.startsWith('# ArchGate sites bundle'))

    const results: {
      slug: string
      ok: boolean
      error?: string
      dry_run?: boolean
    }[] = []

    for (const doc of docs) {
      try {
        const input = parseSiteDocument(doc)
        if (!input.slug) throw new Error('slug ausente no YAML')
        const checked = siteInputSchema.safeParse(input)
        if (!checked.success) {
          throw new Error(checked.error.message)
        }
        const probe = {
          ...checked.data,
          updated_at: new Date().toISOString(),
          updated_by: null,
        } as Site
        assertSiteTenantAccess(probe, s)
        if (data.dry_run) {
          results.push({ slug: input.slug, ok: true, dry_run: true })
          continue
        }
        const site = await upsertSite(checked.data as SiteInput, actor)
        recordActivity(
          'POST',
          `/archgate/sites/${site.slug}/import`,
          actor,
          'success',
          undefined,
          { slug: site.slug },
        )
        results.push({ slug: site.slug, ok: true })
      } catch (e) {
        results.push({
          slug: '?',
          ok: false,
          error: (e as Error).message,
        })
      }
    }

    return {
      results,
      allOk: results.every((r) => r.ok),
      imported: results.filter((r) => r.ok && !r.dry_run).length,
    }
  })

/**
 * Explicit seed of lab stubs — POST only, system:admin.
 * Never called from GET list/get (side-effect free reads).
 */
export const seedDefaultSitesFn = createServerFn({ method: 'POST' }).handler(
  async () => {
    const s = requireSession()
    requireAnyPerm(s, ['system:admin'], 'system:admin')
    const before = (await listSites()).length
    await seedDefaultSitesIfEmpty()
    const after = (await listSites()).length
    const actor = sessionActor(s)
    recordActivity(
      'POST',
      '/archgate/sites/seed',
      actor,
      'success',
      undefined,
      { before, after, seeded: after - before },
    )
    return {
      before,
      after,
      seeded: after - before,
      backend: sitesBackend(),
    }
  },
)

/** Control plane inventory backend (postgres SoT vs sqlite lab). */
export const getSitesBackendFn = createServerFn({ method: 'GET' }).handler(
  async () => {
    requireSession()
    return { backend: sitesBackend() }
  },
)
