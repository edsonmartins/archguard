// Persistence for Org Credential Broker metadata (ADR-013).
// Secrets live only in OpenBao (OCB-1); this table never stores passwords.

import { randomUUID } from 'node:crypto'
import type {
  OrgAccount,
  OrgAccountAuthKind,
  OrgAccountCategory,
  OrgAccountCriticality,
  OrgAccountInput,
} from '@/lib/api/types/org-account'
import { getDb } from './db'
import { logger } from './logger'

type Row = {
  id: string
  slug: string
  name: string
  category: string
  product: string
  url: string
  login_hint: string
  auth_kind: string
  secret_ref: string
  criticality: string
  owner_group: string
  requires_dual_control: number | boolean
  notes: string
  runbook_url: string
  updated_at: string
  updated_by: string | null
}

function normalizeSlug(slug: string): string {
  return slug
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

function rowToAccount(row: Row): OrgAccount {
  return {
    id: row.id,
    slug: row.slug,
    name: row.name,
    category: row.category as OrgAccountCategory,
    product: row.product || '',
    url: row.url || '',
    login_hint: row.login_hint || '',
    auth_kind: row.auth_kind as OrgAccountAuthKind,
    secret_ref: row.secret_ref || '',
    criticality: row.criticality as OrgAccountCriticality,
    owner_group: row.owner_group || '',
    requires_dual_control: !!row.requires_dual_control,
    notes: row.notes || '',
    runbook_url: row.runbook_url || '',
    updated_at: row.updated_at,
    updated_by: row.updated_by,
  }
}

export function listOrgAccounts(): OrgAccount[] {
  const rows = getDb()
    .prepare(
      `SELECT * FROM org_accounts ORDER BY
        CASE criticality WHEN 'P0' THEN 0 WHEN 'P1' THEN 1 ELSE 2 END,
        name COLLATE NOCASE`,
    )
    .all() as Row[]
  return rows.map(rowToAccount)
}

export function getOrgAccount(idOrSlug: string): OrgAccount | null {
  const key = idOrSlug.trim()
  const row = getDb()
    .prepare(`SELECT * FROM org_accounts WHERE id = ? OR slug = ? LIMIT 1`)
    .get(key, key) as Row | undefined
  return row ? rowToAccount(row) : null
}

export function upsertOrgAccount(
  input: OrgAccountInput,
  actor?: string,
): OrgAccount {
  const slug = normalizeSlug(input.slug)
  if (!slug) throw new Error('slug required')
  if (!input.name?.trim()) throw new Error('name required')

  const existing = getOrgAccount(slug)
  const criticality = (input.criticality ||
    existing?.criticality ||
    'P2') as OrgAccountCriticality
  const requiresDual =
    input.requires_dual_control !== undefined
      ? !!input.requires_dual_control
      : existing
        ? existing.requires_dual_control
        : criticality === 'P0'

  const now = new Date().toISOString()
  const id = existing?.id || randomUUID()
  const row: Row = {
    id,
    slug,
    name: input.name.trim(),
    category: input.category || existing?.category || 'other',
    product: (input.product ?? existing?.product ?? '').trim(),
    url: (input.url ?? existing?.url ?? '').trim(),
    login_hint: (input.login_hint ?? existing?.login_hint ?? '').trim(),
    auth_kind: input.auth_kind || existing?.auth_kind || 'password',
    secret_ref: (input.secret_ref ?? existing?.secret_ref ?? '').trim(),
    criticality,
    owner_group: (input.owner_group ?? existing?.owner_group ?? '').trim(),
    requires_dual_control: requiresDual ? 1 : 0,
    notes: (input.notes ?? existing?.notes ?? '').trim(),
    runbook_url: (input.runbook_url ?? existing?.runbook_url ?? '').trim(),
    updated_at: now,
    updated_by: actor || existing?.updated_by || null,
  }

  getDb()
    .prepare(
      `INSERT INTO org_accounts (
        id, slug, name, category, product, url, login_hint, auth_kind,
        secret_ref, criticality, owner_group, requires_dual_control,
        notes, runbook_url, updated_at, updated_by
      ) VALUES (
        @id, @slug, @name, @category, @product, @url, @login_hint, @auth_kind,
        @secret_ref, @criticality, @owner_group, @requires_dual_control,
        @notes, @runbook_url, @updated_at, @updated_by
      )
      ON CONFLICT(slug) DO UPDATE SET
        name = excluded.name,
        category = excluded.category,
        product = excluded.product,
        url = excluded.url,
        login_hint = excluded.login_hint,
        auth_kind = excluded.auth_kind,
        secret_ref = excluded.secret_ref,
        criticality = excluded.criticality,
        owner_group = excluded.owner_group,
        requires_dual_control = excluded.requires_dual_control,
        notes = excluded.notes,
        runbook_url = excluded.runbook_url,
        updated_at = excluded.updated_at,
        updated_by = excluded.updated_by`,
    )
    .run(row)

  logger.info({ slug, actor }, 'org account upserted')
  return getOrgAccount(slug)!
}

export function deleteOrgAccount(idOrSlug: string): boolean {
  const acc = getOrgAccount(idOrSlug)
  if (!acc) return false
  getDb().prepare(`DELETE FROM org_accounts WHERE id = ?`).run(acc.id)
  logger.info({ slug: acc.slug }, 'org account deleted')
  return true
}

/** Seed IntegrAllTech inventory placeholders (no secrets). */
export function seedDefaultOrgAccountsIfEmpty(actor = 'system'): number {
  const n = (
    getDb().prepare(`SELECT COUNT(*) AS c FROM org_accounts`).get() as {
      c: number
    }
  ).c
  if (n > 0) return 0

  const defaults: OrgAccountInput[] = [
    {
      slug: 'apple-appstore-integrall',
      name: 'App Store Connect · IntegrAllTech',
      category: 'store',
      criticality: 'P0',
      url: 'https://appstoreconnect.apple.com',
      auth_kind: 'password',
      owner_group: 'archguard_super_admins',
      secret_ref: 'secret/data/org/store/apple-appstore',
      notes: 'Owner / dual-control. Prefer individual users + API key.',
    },
    {
      slug: 'google-play-integrall',
      name: 'Google Play Console · IntegrAllTech',
      category: 'store',
      criticality: 'P0',
      url: 'https://play.google.com/console',
      auth_kind: 'password',
      owner_group: 'archguard_super_admins',
      secret_ref: 'secret/data/org/store/google-play',
      notes: 'Prefer IAM individual; shared owner break-glass only.',
    },
    {
      slug: 'gcp-integrall-placeholder',
      name: 'Google Cloud · (preencher projeto)',
      category: 'cloud',
      criticality: 'P0',
      url: 'https://console.cloud.google.com',
      auth_kind: 'oidc',
      owner_group: 'archguard_super_admins',
      secret_ref: 'secret/data/org/cloud/gcp-placeholder',
      notes: 'Prefer Workspace SSO + SA keys in OpenBao; not shared password.',
    },
    {
      slug: 'vendax-admin',
      name: 'Vendax · admin',
      category: 'product_admin',
      product: 'vendax',
      criticality: 'P1',
      auth_kind: 'password',
      owner_group: 'archguard_users',
      secret_ref: 'secret/data/org/product/vendax-admin',
    },
    {
      slug: 'archflow-admin',
      name: 'ArchFlow · admin',
      category: 'product_admin',
      product: 'archflow',
      criticality: 'P1',
      auth_kind: 'password',
      owner_group: 'archguard_users',
      secret_ref: 'secret/data/org/product/archflow-admin',
    },
    {
      slug: 'archguard-admin-breakglass',
      name: 'ArchGuard · break-glass (se houver)',
      category: 'product_admin',
      product: 'archguard',
      criticality: 'P0',
      auth_kind: 'password',
      owner_group: 'archguard_super_admins',
      secret_ref: 'secret/data/org/product/archguard-breakglass',
      notes: 'Prefer SSO Kanidm; password only break-glass.',
    },
    {
      slug: 'brainsentry-admin',
      name: 'BrainSentry · admin',
      category: 'product_admin',
      product: 'brainsentry',
      criticality: 'P1',
      auth_kind: 'password',
      owner_group: 'archguard_users',
      secret_ref: 'secret/data/org/product/brainsentry-admin',
    },
    {
      slug: 'gestor-admin',
      name: 'Gestor RQ · admin',
      category: 'product_admin',
      product: 'gestor',
      criticality: 'P1',
      auth_kind: 'password',
      owner_group: 'archguard_users',
      secret_ref: 'secret/data/org/product/gestor-admin',
    },
    {
      slug: 'alcada-admin',
      name: 'Alçada · admin',
      category: 'product_admin',
      product: 'alcada',
      criticality: 'P1',
      auth_kind: 'password',
      owner_group: 'archguard_users',
      secret_ref: 'secret/data/org/product/alcada-admin',
    },
  ]

  for (const d of defaults) {
    upsertOrgAccount(d, actor)
  }
  logger.info({ count: defaults.length }, 'seeded default org accounts')
  return defaults.length
}
