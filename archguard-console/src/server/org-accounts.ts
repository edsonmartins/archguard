// Persistence for Org Credential Broker metadata (ADR-013).
// Secrets live only in OpenBao (OCB-1); this table never stores passwords.

import { randomUUID } from 'node:crypto'
import type {
  OrgAccount,
  OrgAccountAuthKind,
  OrgAccountCategory,
  OrgAccountCriticality,
  OrgAccountInput,
  OrgFederationStatus,
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
  federation_status?: string | null
  oidc_client_id?: string | null
  secret_ref: string
  criticality: string
  owner_group: string
  requires_dual_control: number | boolean
  notes: string
  runbook_url: string
  rotated_at: string | null
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
    federation_status: (row.federation_status ||
      'password_only') as OrgFederationStatus,
    oidc_client_id: row.oidc_client_id || '',
    secret_ref: row.secret_ref || '',
    criticality: row.criticality as OrgAccountCriticality,
    owner_group: row.owner_group || '',
    requires_dual_control: !!row.requires_dual_control,
    notes: row.notes || '',
    runbook_url: row.runbook_url || '',
    rotated_at: row.rotated_at || null,
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
    federation_status:
      input.federation_status ||
      existing?.federation_status ||
      'password_only',
    oidc_client_id: (
      input.oidc_client_id ??
      existing?.oidc_client_id ??
      ''
    ).trim(),
    secret_ref: (input.secret_ref ?? existing?.secret_ref ?? '').trim(),
    criticality,
    owner_group: (input.owner_group ?? existing?.owner_group ?? '').trim(),
    requires_dual_control: requiresDual ? 1 : 0,
    notes: (input.notes ?? existing?.notes ?? '').trim(),
    runbook_url: (input.runbook_url ?? existing?.runbook_url ?? '').trim(),
    rotated_at: existing?.rotated_at ?? null,
    updated_at: now,
    updated_by: actor || existing?.updated_by || null,
  }

  getDb()
    .prepare(
      `INSERT INTO org_accounts (
        id, slug, name, category, product, url, login_hint, auth_kind,
        federation_status, oidc_client_id,
        secret_ref, criticality, owner_group, requires_dual_control,
        notes, runbook_url, rotated_at, updated_at, updated_by
      ) VALUES (
        @id, @slug, @name, @category, @product, @url, @login_hint, @auth_kind,
        @federation_status, @oidc_client_id,
        @secret_ref, @criticality, @owner_group, @requires_dual_control,
        @notes, @runbook_url, @rotated_at, @updated_at, @updated_by
      )
      ON CONFLICT(slug) DO UPDATE SET
        name = excluded.name,
        category = excluded.category,
        product = excluded.product,
        url = excluded.url,
        login_hint = excluded.login_hint,
        auth_kind = excluded.auth_kind,
        federation_status = excluded.federation_status,
        oidc_client_id = excluded.oidc_client_id,
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

/** Mark last secret rotation time (secret material only in OpenBao). */
export function markOrgAccountRotated(
  idOrSlug: string,
  actor?: string,
): OrgAccount | null {
  const acc = getOrgAccount(idOrSlug)
  if (!acc) return null
  const now = new Date().toISOString()
  getDb()
    .prepare(
      `UPDATE org_accounts
       SET rotated_at = ?, updated_at = ?, updated_by = ?
       WHERE id = ?`,
    )
    .run(now, now, actor || acc.updated_by, acc.id)
  return getOrgAccount(acc.id)
}

/** Canonical IntegrAllTech inventory placeholders (no secrets). Shared by seed + OCB-4 backfill. */
export function defaultOrgAccountSpecs(): OrgAccountInput[] {
  return [
    {
      slug: 'apple-appstore-integrall',
      name: 'App Store Connect · IntegrAllTech',
      category: 'store',
      criticality: 'P0',
      url: 'https://appstoreconnect.apple.com',
      auth_kind: 'api_key',
      federation_status: 'api_key_primary',
      owner_group: 'archguard_super_admins',
      secret_ref: 'secret/data/org/store/apple-appstore',
      runbook_url: 'documentos/runbooks/org-product-oidc-federation.md',
      notes:
        'OCB-4: usuários individuais + App Store Connect API key no OpenBao; Apple ID owner dual-control break-glass.',
    },
    {
      slug: 'google-play-integrall',
      name: 'Google Play Console · IntegrAllTech',
      category: 'store',
      criticality: 'P0',
      url: 'https://play.google.com/console',
      auth_kind: 'api_key',
      federation_status: 'external_idp',
      owner_group: 'archguard_super_admins',
      secret_ref: 'secret/data/org/store/google-play',
      runbook_url: 'documentos/runbooks/org-product-oidc-federation.md',
      notes:
        'OCB-4: Google individual IAM; service account / API no OpenBao; owner compartilhado só break-glass.',
    },
    {
      slug: 'gcp-integrall-placeholder',
      name: 'Google Cloud · (preencher projeto)',
      category: 'cloud',
      criticality: 'P0',
      url: 'https://console.cloud.google.com',
      auth_kind: 'oidc',
      federation_status: 'external_idp',
      owner_group: 'archguard_super_admins',
      secret_ref: 'secret/data/org/cloud/gcp-placeholder',
      runbook_url: 'documentos/runbooks/org-product-oidc-federation.md',
      notes:
        'Workspace/SSO Google + SA keys no OpenBao. Sem senha compartilhada no dia a dia.',
    },
    {
      slug: 'vendax-admin',
      name: 'Vendax · admin (break-glass)',
      category: 'product_admin',
      product: 'vendax',
      criticality: 'P1',
      auth_kind: 'oidc',
      federation_status: 'oidc_primary',
      oidc_client_id: 'vendax-admin',
      owner_group: 'archguard_users',
      secret_ref: 'secret/data/org/product/vendax-admin',
      runbook_url: 'documentos/runbooks/org-product-oidc-federation.md',
      notes:
        'Meta: SSO Kanidm. Password só break-glass dual-control até OIDC live.',
    },
    {
      slug: 'archflow-admin',
      name: 'ArchFlow · admin (break-glass)',
      category: 'product_admin',
      product: 'archflow',
      criticality: 'P1',
      auth_kind: 'oidc',
      federation_status: 'oidc_primary',
      oidc_client_id: 'archflow-admin',
      owner_group: 'archguard_users',
      secret_ref: 'secret/data/org/product/archflow-admin',
      runbook_url: 'documentos/runbooks/org-product-oidc-federation.md',
      notes: 'Meta: SSO Kanidm; senha só break-glass.',
    },
    {
      slug: 'archguard-admin-breakglass',
      name: 'ArchGuard/Manager · break-glass',
      category: 'product_admin',
      product: 'archguard',
      criticality: 'P0',
      auth_kind: 'oidc',
      federation_status: 'oidc_primary',
      oidc_client_id: 'archguard-console',
      owner_group: 'archguard_super_admins',
      secret_ref: 'secret/data/org/product/archguard-breakglass',
      runbook_url: 'documentos/runbooks/org-product-oidc-federation.md',
      notes:
        'Já usa Kanidm OIDC (archguard-console). Secret = break-glass local se existir.',
    },
    {
      slug: 'brainsentry-admin',
      name: 'BrainSentry · admin (break-glass)',
      category: 'product_admin',
      product: 'brainsentry',
      criticality: 'P1',
      auth_kind: 'oidc',
      federation_status: 'oidc_primary',
      oidc_client_id: 'brainsentry-admin',
      owner_group: 'archguard_users',
      secret_ref: 'secret/data/org/product/brainsentry-admin',
      runbook_url: 'documentos/runbooks/org-product-oidc-federation.md',
    },
    {
      slug: 'gestor-admin',
      name: 'Gestor RQ · admin (break-glass)',
      category: 'product_admin',
      product: 'gestor',
      criticality: 'P1',
      auth_kind: 'oidc',
      federation_status: 'oidc_primary',
      oidc_client_id: 'gestor-admin',
      owner_group: 'archguard_users',
      secret_ref: 'secret/data/org/product/gestor-admin',
      runbook_url: 'documentos/runbooks/org-product-oidc-federation.md',
    },
    {
      slug: 'alcada-admin',
      name: 'Alçada · admin (break-glass)',
      category: 'product_admin',
      product: 'alcada',
      criticality: 'P1',
      auth_kind: 'oidc',
      federation_status: 'oidc_primary',
      oidc_client_id: 'alcada-admin',
      owner_group: 'archguard_users',
      secret_ref: 'secret/data/org/product/alcada-admin',
      runbook_url: 'documentos/runbooks/org-product-oidc-federation.md',
    },
  ]
}

/** Seed IntegrAllTech inventory placeholders (no secrets). */
export function seedDefaultOrgAccountsIfEmpty(actor = 'system'): number {
  const n = (
    getDb().prepare(`SELECT COUNT(*) AS c FROM org_accounts`).get() as {
      c: number
    }
  ).c
  if (n > 0) return 0

  const defaults = defaultOrgAccountSpecs()
  for (const d of defaults) {
    upsertOrgAccount(d, actor)
  }
  logger.info({ count: defaults.length }, 'seeded default org accounts')
  return defaults.length
}

/**
 * OCB-4 live backfill: for known seed slugs still on password_only (migration
 * default), apply federation_status / oidc_client_id / auth_kind from specs.
 * Does not overwrite oidc_only or other explicit federation choices.
 * Idempotent — returns number of rows updated.
 */
export function backfillOrgAccountFederation(actor = 'system'): number {
  let updated = 0
  for (const d of defaultOrgAccountSpecs()) {
    if (!d.slug) continue
    const existing = getOrgAccount(d.slug)
    if (!existing) continue

    const fed = existing.federation_status || 'password_only'
    const needsFed = fed === 'password_only' && d.federation_status
    const needsOidc =
      !!d.oidc_client_id && !existing.oidc_client_id && fed !== 'oidc_only'
    if (!needsFed && !needsOidc) continue

    upsertOrgAccount(
      {
        slug: d.slug,
        ...(needsFed
          ? {
              name: d.name,
              auth_kind: d.auth_kind,
              federation_status: d.federation_status,
              runbook_url: d.runbook_url || existing.runbook_url,
              notes: existing.notes?.trim() ? existing.notes : d.notes,
            }
          : {}),
        ...(needsOidc || needsFed
          ? { oidc_client_id: d.oidc_client_id || existing.oidc_client_id }
          : {}),
      },
      actor,
    )
    updated++
  }
  if (updated > 0) {
    logger.info({ count: updated }, 'backfilled org account federation metadata')
  }
  return updated
}
