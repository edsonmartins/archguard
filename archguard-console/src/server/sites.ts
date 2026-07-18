// Persistence for ArchGate client/site inventory.
// SoT: PostgreSQL when CONSOLE_DATABASE_URL (or ARCHGUARD_DATABASE_URL) is set;
// otherwise SQLite (lab / single-node fallback with volume).

import type {
  Site,
  SiteConnector,
  SiteInput,
  SiteStackMeta,
  SiteTarget,
} from '@/lib/api/types/site'
import {
  normalizeSiteConnectors,
  primaryStackFromConnectors,
} from '@/lib/api/types/site'
import { getDb } from './db'
import { logger } from './logger'

export type SiteRow = {
  slug: string
  cliente: string
  tenant_group: string
  ambiente: string
  tipo: string
  stack: string
  connector_id: string | null
  subnets_json: string
  stack_meta_json: string
  targets_json: string
  warpgate_roles_json: string
  /** Optional multi-connector JSON; missing column → legacy single connector. */
  connectors_json?: string | null
  notas: string | null
  inventariado: number | boolean
  connector_deployed: number | boolean
  smoke_operador: number | boolean
  updated_at: string
  updated_by: string | null
}

function parseJson<T>(raw: string, fallback: T): T {
  try {
    return JSON.parse(raw) as T
  } catch {
    return fallback
  }
}

function rowToSite(row: SiteRow): Site {
  const base = {
    slug: row.slug,
    cliente: row.cliente,
    tenant_group: row.tenant_group,
    ambiente: row.ambiente as Site['ambiente'],
    tipo: row.tipo as Site['tipo'],
    stack: row.stack as Site['stack'],
    connector_id: row.connector_id || '',
    subnets: parseJson<string[]>(row.subnets_json, []),
    stack_meta: parseJson<SiteStackMeta>(row.stack_meta_json, {}),
    connectors: parseJson<SiteConnector[]>(row.connectors_json || '[]', []),
    targets: parseJson<SiteTarget[]>(row.targets_json, []),
    warpgate_roles: parseJson<string[]>(row.warpgate_roles_json, []),
    notas: row.notas || '',
    inventariado: !!row.inventariado,
    connector_deployed: !!row.connector_deployed,
    smoke_operador: !!row.smoke_operador,
    updated_at: row.updated_at,
    updated_by: row.updated_by,
  }
  const connectors = normalizeSiteConnectors(base)
  return {
    ...base,
    connectors,
    // Keep legacy primary fields aligned with first connector when multi.
    connector_id: connectors[0]?.id || base.connector_id,
    stack: primaryStackFromConnectors(connectors, base.stack),
    tipo: (connectors[0]?.tipo as Site['tipo']) || base.tipo,
    subnets:
      connectors.length > 1
        ? Array.from(
            new Set(connectors.flatMap((c) => c.subnets || [])),
          )
        : connectors[0]?.subnets?.length
          ? connectors[0].subnets
          : base.subnets,
  }
}

function normalizeSlug(slug: string): string {
  return slug
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '_')
    .replace(/^_+|_+$/g, '')
}

/**
 * Resolve the Postgres connection string for the sites SoT.
 * Only real postgres URLs count — `file:` / sqlite paths must NOT trigger PG
 * (common misconfig: DATABASE_URL=file:/data/console.sqlite from generic templates).
 */
function consoleDatabaseUrl(): string {
  const raw = (
    process.env.CONSOLE_DATABASE_URL ||
    process.env.ARCHGUARD_DATABASE_URL ||
    process.env.DATABASE_URL ||
    ''
  ).trim()
  if (!raw) return ''
  // Explicit non-postgres schemes → sqlite path elsewhere (ARCHGUARD_DB_PATH)
  if (/^(file:|sqlite:)/i.test(raw)) return ''
  if (/^postgres(ql)?:\/\//i.test(raw)) return raw
  // Bare host strings or other schemes are not treated as Postgres
  return ''
}

export function sitesBackend(): 'postgres' | 'sqlite' {
  return consoleDatabaseUrl() ? 'postgres' : 'sqlite'
}

// --- PostgreSQL ---

type PgPool = import('pg').Pool
let pgPool: PgPool | null = null
let pgReady: Promise<void> | null = null

async function getPg(): Promise<PgPool> {
  if (pgPool) return pgPool
  const { default: pg } = await import('pg')
  const url = consoleDatabaseUrl()
  if (!url) throw new Error('CONSOLE_DATABASE_URL not set')
  pgPool = new pg.Pool({
    connectionString: url,
    max: 5,
    idleTimeoutMillis: 30_000,
    connectionTimeoutMillis: 8_000,
  })
  pgPool.on('error', (err) => {
    logger.error({ err: String(err) }, 'postgres pool error')
  })
  return pgPool
}

async function ensurePgSchema(): Promise<void> {
  if (pgReady) return pgReady
  pgReady = (async () => {
    const pool = await getPg()
    await pool.query(`
      CREATE TABLE IF NOT EXISTS sites (
        slug            TEXT PRIMARY KEY,
        cliente         TEXT NOT NULL,
        tenant_group    TEXT NOT NULL,
        ambiente        TEXT NOT NULL DEFAULT 'producao',
        tipo            TEXT NOT NULL DEFAULT 'a_confirmar',
        stack           TEXT NOT NULL DEFAULT 'a_confirmar',
        connector_id    TEXT,
        subnets_json    TEXT NOT NULL DEFAULT '[]',
        stack_meta_json TEXT NOT NULL DEFAULT '{}',
        connectors_json TEXT NOT NULL DEFAULT '[]',
        targets_json    TEXT NOT NULL DEFAULT '[]',
        warpgate_roles_json TEXT NOT NULL DEFAULT '[]',
        notas           TEXT,
        inventariado    BOOLEAN NOT NULL DEFAULT FALSE,
        connector_deployed BOOLEAN NOT NULL DEFAULT FALSE,
        smoke_operador  BOOLEAN NOT NULL DEFAULT FALSE,
        updated_at      TIMESTAMPTZ NOT NULL,
        updated_by      TEXT
      );
      CREATE INDEX IF NOT EXISTS idx_sites_stack ON sites (stack);
      CREATE INDEX IF NOT EXISTS idx_sites_tenant ON sites (tenant_group);
    `)
    await ensurePgConnectorsColumn()
    logger.info('sites SoT: PostgreSQL schema ready')
  })()
  return pgReady
}

let pgConnectorsReady = false
async function ensurePgConnectorsColumn(): Promise<void> {
  if (pgConnectorsReady) return
  const pool = await getPg()
  await pool.query(`
    ALTER TABLE sites
      ADD COLUMN IF NOT EXISTS connectors_json TEXT NOT NULL DEFAULT '[]'
  `)
  pgConnectorsReady = true
}

let sqliteConnectorsReady = false
function ensureSqliteConnectorsColumn(): void {
  if (sqliteConnectorsReady) return
  const db = getDb()
  const cols = db.prepare(`PRAGMA table_info(sites)`).all() as { name: string }[]
  if (!cols.some((c) => c.name === 'connectors_json')) {
    db.exec(
      `ALTER TABLE sites ADD COLUMN connectors_json TEXT NOT NULL DEFAULT '[]'`,
    )
  }
  sqliteConnectorsReady = true
}

// --- public API (async) ---

export async function listSites(): Promise<Site[]> {
  if (sitesBackend() === 'postgres') {
    await ensurePgSchema()
    const pool = await getPg()
    const { rows } = await pool.query<SiteRow>(
      'SELECT * FROM sites ORDER BY lower(cliente) ASC',
    )
    return rows.map(rowToSite)
  }
  ensureSqliteConnectorsColumn()
  const rows = getDb()
    .prepare('SELECT * FROM sites ORDER BY cliente COLLATE NOCASE ASC')
    .all() as SiteRow[]
  return rows.map(rowToSite)
}

export async function getSite(slug: string): Promise<Site | null> {
  if (sitesBackend() === 'postgres') {
    await ensurePgSchema()
    const pool = await getPg()
    const { rows } = await pool.query<SiteRow>(
      'SELECT * FROM sites WHERE slug = $1',
      [slug],
    )
    return rows[0] ? rowToSite(rows[0]) : null
  }
  ensureSqliteConnectorsColumn()
  const row = getDb()
    .prepare('SELECT * FROM sites WHERE slug = ?')
    .get(slug) as SiteRow | undefined
  return row ? rowToSite(row) : null
}

export async function upsertSite(
  input: SiteInput,
  actor?: string | null,
): Promise<Site> {
  const slug = normalizeSlug(input.slug)
  if (!slug) throw new Error('slug obrigatório')
  if (!input.cliente?.trim()) throw new Error('cliente obrigatório')

  const tenant =
    input.tenant_group?.trim() ||
    (slug.startsWith('tenant_') ? slug : `tenant_${slug}`)

  const now = new Date().toISOString()
  const connectors = normalizeSiteConnectors({
    slug,
    tipo: input.tipo || 'a_confirmar',
    stack: input.stack || 'a_confirmar',
    connector_id: input.connector_id || `connector-${slug}`,
    subnets: input.subnets || [],
    stack_meta: input.stack_meta || {},
    connectors: input.connectors || [],
  })
  const primary = connectors[0]
  const stack = primaryStackFromConnectors(
    connectors,
    (input.stack as Site['stack']) || 'a_confirmar',
  )
  const params = {
    slug,
    cliente: input.cliente.trim(),
    tenant_group: tenant,
    ambiente: input.ambiente || 'producao',
    tipo: primary?.tipo || input.tipo || 'a_confirmar',
    stack,
    connector_id: primary?.id || input.connector_id || `connector-${slug}`,
    subnets_json: JSON.stringify(
      primary?.subnets?.length
        ? primary.subnets
        : input.subnets || [],
    ),
    stack_meta_json: JSON.stringify(
      primary?.meta && Object.keys(primary.meta).length
        ? primary.meta
        : input.stack_meta || {},
    ),
    connectors_json: JSON.stringify(connectors),
    targets_json: JSON.stringify(input.targets || []),
    warpgate_roles_json: JSON.stringify(input.warpgate_roles || []),
    notas: input.notas || '',
    inventariado: !!input.inventariado,
    connector_deployed: !!input.connector_deployed,
    smoke_operador: !!input.smoke_operador,
    updated_at: now,
    updated_by: actor || input.updated_by || null,
  }

  if (sitesBackend() === 'postgres') {
    await ensurePgSchema()
    const pool = await getPg()
    await ensurePgConnectorsColumn()
    await pool.query(
      `INSERT INTO sites (
        slug, cliente, tenant_group, ambiente, tipo, stack, connector_id,
        subnets_json, stack_meta_json, connectors_json, targets_json, warpgate_roles_json, notas,
        inventariado, connector_deployed, smoke_operador, updated_at, updated_by
      ) VALUES (
        $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18
      )
      ON CONFLICT (slug) DO UPDATE SET
        cliente=EXCLUDED.cliente,
        tenant_group=EXCLUDED.tenant_group,
        ambiente=EXCLUDED.ambiente,
        tipo=EXCLUDED.tipo,
        stack=EXCLUDED.stack,
        connector_id=EXCLUDED.connector_id,
        subnets_json=EXCLUDED.subnets_json,
        stack_meta_json=EXCLUDED.stack_meta_json,
        connectors_json=EXCLUDED.connectors_json,
        targets_json=EXCLUDED.targets_json,
        warpgate_roles_json=EXCLUDED.warpgate_roles_json,
        notas=EXCLUDED.notas,
        inventariado=EXCLUDED.inventariado,
        connector_deployed=EXCLUDED.connector_deployed,
        smoke_operador=EXCLUDED.smoke_operador,
        updated_at=EXCLUDED.updated_at,
        updated_by=EXCLUDED.updated_by
      `,
      [
        params.slug,
        params.cliente,
        params.tenant_group,
        params.ambiente,
        params.tipo,
        params.stack,
        params.connector_id,
        params.subnets_json,
        params.stack_meta_json,
        params.connectors_json,
        params.targets_json,
        params.warpgate_roles_json,
        params.notas,
        params.inventariado,
        params.connector_deployed,
        params.smoke_operador,
        params.updated_at,
        params.updated_by,
      ],
    )
  } else {
    ensureSqliteConnectorsColumn()
    getDb()
      .prepare(
        `INSERT INTO sites (
      slug, cliente, tenant_group, ambiente, tipo, stack, connector_id,
      subnets_json, stack_meta_json, connectors_json, targets_json, warpgate_roles_json, notas,
      inventariado, connector_deployed, smoke_operador, updated_at, updated_by
    ) VALUES (
      @slug, @cliente, @tenant_group, @ambiente, @tipo, @stack, @connector_id,
      @subnets_json, @stack_meta_json, @connectors_json, @targets_json, @warpgate_roles_json, @notas,
      @inventariado, @connector_deployed, @smoke_operador, @updated_at, @updated_by
    )
    ON CONFLICT(slug) DO UPDATE SET
      cliente=excluded.cliente,
      tenant_group=excluded.tenant_group,
      ambiente=excluded.ambiente,
      tipo=excluded.tipo,
      stack=excluded.stack,
      connector_id=excluded.connector_id,
      subnets_json=excluded.subnets_json,
      stack_meta_json=excluded.stack_meta_json,
      connectors_json=excluded.connectors_json,
      targets_json=excluded.targets_json,
      warpgate_roles_json=excluded.warpgate_roles_json,
      notas=excluded.notas,
      inventariado=excluded.inventariado,
      connector_deployed=excluded.connector_deployed,
      smoke_operador=excluded.smoke_operador,
      updated_at=excluded.updated_at,
      updated_by=excluded.updated_by
    `,
      )
      .run({
        ...params,
        inventariado: params.inventariado ? 1 : 0,
        connector_deployed: params.connector_deployed ? 1 : 0,
        smoke_operador: params.smoke_operador ? 1 : 0,
      })
  }

  const site = await getSite(slug)
  if (!site) throw new Error('falha ao gravar site')
  return site
}

export async function deleteSite(slug: string): Promise<boolean> {
  if (sitesBackend() === 'postgres') {
    await ensurePgSchema()
    const pool = await getPg()
    const r = await pool.query('DELETE FROM sites WHERE slug = $1', [slug])
    return (r.rowCount ?? 0) > 0
  }
  const r = getDb().prepare('DELETE FROM sites WHERE slug = ?').run(slug)
  return r.changes > 0
}

/** Seed lab/prod stubs if table empty (idempotent). */
export async function seedDefaultSitesIfEmpty(): Promise<void> {
  if ((await listSites()).length > 0) return
  const seeds: SiteInput[] = [
    {
      slug: 'rio_quality_lab',
      cliente: 'Rio Quality (lab)',
      tenant_group: 'tenant_rio_quality',
      ambiente: 'lab',
      tipo: 'tunnel_agent',
      stack: 'lab_overlay',
      connector_id: 'lab-runtime',
      subnets: ['10.0.2.0/24'],
      stack_meta: {},
      connectors: [],
      targets: [
        {
          nome: 'rio-lab-ssh',
          engine: 'warpgate',
          protocolo: 'ssh',
          host: '10.0.2.99',
          port: 2222,
          roles: ['tenant-rio-quality'],
        },
        {
          nome: 'rio-lab-postgres',
          engine: 'warpgate',
          protocolo: 'postgres',
          host: '192.168.1.110',
          port: 5434,
          roles: ['tenant-rio-quality'],
        },
      ],
      warpgate_roles: ['tenant-rio-quality'],
      notas: 'Lab IntegrAllTech — não é VPN de produção',
      inventariado: true,
      connector_deployed: true,
      smoke_operador: true,
    },
    {
      slug: 'grupo_marra_lab',
      cliente: 'Grupo Marra (lab)',
      tenant_group: 'tenant_grupo_marra',
      ambiente: 'lab',
      tipo: 'tunnel_agent',
      stack: 'lab_overlay',
      connector_id: 'lab-runtime',
      subnets: ['10.0.2.0/24'],
      stack_meta: {},
      connectors: [],
      targets: [
        {
          nome: 'marra-lab-ssh',
          engine: 'warpgate',
          protocolo: 'ssh',
          host: '10.0.2.99',
          port: 2222,
          roles: ['tenant-grupo-marra'],
        },
      ],
      warpgate_roles: ['tenant-grupo-marra'],
      notas: 'Lab IntegrAllTech',
      inventariado: true,
      connector_deployed: true,
      smoke_operador: true,
    },
    {
      slug: 'rio_quality',
      cliente: 'Rio Quality',
      tenant_group: 'tenant_rio_quality',
      ambiente: 'producao',
      tipo: 'tunnel_agent',
      stack: 'mixed',
      connector_id: 'connector-rio-forti',
      subnets: [],
      stack_meta: {},
      connectors: [
        {
          id: 'connector-rio-forti',
          stack: 'openfortivpn',
          tipo: 'tunnel_agent',
          subnets: [],
          meta: { forti_host: 'a_confirmar' },
          notas: 'LAN Rio — Oracle / ERP / Linux',
        },
        {
          id: 'connector-rio-aws',
          stack: 'openvpn',
          tipo: 'tunnel_agent',
          subnets: [],
          meta: { ovpn_path: 'a_confirmar' },
          notas: 'AWS — APIs / DBs / observabilidade',
        },
      ],
      targets: [],
      warpgate_roles: ['tenant-rio-quality'],
      notas:
        'Produção — dual VPN: openfortivpn (LAN) + OpenVPN (AWS). Peer real = calendário H2.',
      inventariado: false,
      connector_deployed: false,
      smoke_operador: false,
    },
    {
      slug: 'grupo_marra',
      cliente: 'Grupo Marra',
      tenant_group: 'tenant_grupo_marra',
      ambiente: 'producao',
      tipo: 'a_confirmar',
      stack: 'a_confirmar',
      connector_id: 'connector-grupo-marra',
      subnets: [],
      stack_meta: {},
      connectors: [],
      targets: [],
      warpgate_roles: ['tenant-grupo-marra'],
      notas: 'Produção — confirmar stack OpenVPN / WireGuard / openfortivpn',
      inventariado: false,
      connector_deployed: false,
      smoke_operador: false,
    },
  ]
  for (const s of seeds) {
    await upsertSite(s, 'seed')
  }
}
