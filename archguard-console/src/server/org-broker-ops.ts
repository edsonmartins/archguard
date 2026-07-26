// Org Credential Broker — Manager-only ops (settings + health + ensure write).
// ADR-013 C7 / ADR-009: admin never opens OpenBao/Kanidm UIs for day-to-day.

import { listOrgAccounts } from './org-accounts'
import { getSetting, setSetting } from './manager-settings'
import {
  getHealth,
  openbaoConfigured,
  openbaoTokenConfigured,
  openbaoTokenKind,
  writeSecretData,
  readSecretData,
} from './openbao-proxy'
import {
  TTL_DEFAULT,
  TTL_MAX,
  TTL_MIN,
} from './org-checkouts'

const KEY_WEBHOOK = 'org.checkout_webhook_url'
const KEY_TTL_DEFAULT = 'org.ttl_default_seconds'
const KEY_TTL_MAX = 'org.ttl_max_seconds'

/** Canary path used only by health probe (no customer secrets). */
export const ORG_HEALTH_SECRET_REF = 'secret/data/org/_archgate_health'

export type OrgBrokerSettings = {
  checkout_webhook_url: string
  /** True when webhook comes from env bootstrap (read-only fallback shown in UI). */
  webhook_from_env: boolean
  ttl_default_seconds: number
  ttl_max_seconds: number
  ttl_min_seconds: number
}

export type OrgBrokerHealth = {
  ok: boolean
  openbao: {
    address_configured: boolean
    token_configured: boolean
    token_kind: 'app' | 'root' | 'none'
    reachable: boolean
    sealed: boolean | null
    write_ok: boolean
    detail?: string
  }
  settings: {
    webhook_configured: boolean
    ttl_default_seconds: number
    ttl_max_seconds: number
  }
  accounts: {
    total: number
    with_secret_ref: number
    p0_missing_secret_ref: number
  }
  /** Operator-facing hints (Manager language — no "open OpenBao UI"). */
  hints: string[]
}

function parseIntClamped(
  raw: string,
  fallback: number,
  min: number,
  max: number,
): number {
  const n = Number.parseInt(raw, 10)
  if (!Number.isFinite(n)) return fallback
  return Math.min(max, Math.max(min, n))
}

export function getOrgBrokerSettings(): OrgBrokerSettings {
  const dbUrl = getSetting(KEY_WEBHOOK).trim()
  const envUrl = (process.env.ORG_CHECKOUT_WEBHOOK_URL || '').trim()
  const webhook = dbUrl || envUrl
  const ttlMax = parseIntClamped(
    getSetting(KEY_TTL_MAX),
    TTL_MAX,
    TTL_MIN,
    24 * 3600,
  )
  const ttlDefault = parseIntClamped(
    getSetting(KEY_TTL_DEFAULT),
    TTL_DEFAULT,
    TTL_MIN,
    ttlMax,
  )
  return {
    checkout_webhook_url: webhook,
    webhook_from_env: !dbUrl && !!envUrl,
    ttl_default_seconds: ttlDefault,
    ttl_max_seconds: ttlMax,
    ttl_min_seconds: TTL_MIN,
  }
}

/** Resolve webhook URL: Manager setting first, then bootstrap env. */
export function resolveCheckoutWebhookUrl(): string {
  const dbUrl = getSetting(KEY_WEBHOOK).trim()
  if (dbUrl) return dbUrl
  return (process.env.ORG_CHECKOUT_WEBHOOK_URL || '').trim()
}

export function saveOrgBrokerSettings(
  input: {
    checkout_webhook_url?: string
    ttl_default_seconds?: number
    ttl_max_seconds?: number
  },
  actor: string,
): OrgBrokerSettings {
  if (input.checkout_webhook_url !== undefined) {
    const url = input.checkout_webhook_url.trim()
    if (url && !/^https:\/\//i.test(url)) {
      throw new Error('Webhook deve ser HTTPS (https://…)')
    }
    setSetting(KEY_WEBHOOK, url, actor)
  }
  if (input.ttl_max_seconds !== undefined) {
    const max = Math.min(
      24 * 3600,
      Math.max(TTL_MIN, Math.floor(input.ttl_max_seconds)),
    )
    setSetting(KEY_TTL_MAX, String(max), actor)
  }
  if (input.ttl_default_seconds !== undefined) {
    const cur = getOrgBrokerSettings()
    const max = cur.ttl_max_seconds
    const def = Math.min(
      max,
      Math.max(TTL_MIN, Math.floor(input.ttl_default_seconds)),
    )
    setSetting(KEY_TTL_DEFAULT, String(def), actor)
  }
  return getOrgBrokerSettings()
}

/**
 * Probe secrets backend from Manager (no browser token).
 * Writes a tiny canary under secret/data/org/_archgate_health.
 */
export async function probeOrgSecretBackend(): Promise<{
  write_ok: boolean
  detail?: string
  sealed: boolean | null
  reachable: boolean
}> {
  if (!openbaoConfigured()) {
    return {
      write_ok: false,
      reachable: false,
      sealed: null,
      detail: 'Backend de segredos não configurado no console (bootstrap de plataforma).',
    }
  }
  if (!openbaoTokenConfigured()) {
    return {
      write_ok: false,
      reachable: true,
      sealed: null,
      detail:
        'Token de aplicação do backend de segredos ausente. Contate admin de plataforma.',
    }
  }

  let sealed: boolean | null = null
  let reachable = false
  try {
    const h = await getHealth()
    reachable = h.http_status > 0 && h.http_status < 500
    sealed = typeof h.sealed === 'boolean' ? h.sealed : null
    if (h.sealed) {
      return {
        write_ok: false,
        reachable: true,
        sealed: true,
        detail: 'Backend de segredos está selado — admin de plataforma deve unseal.',
      }
    }
  } catch (e) {
    return {
      write_ok: false,
      reachable: false,
      sealed: null,
      detail: `Backend de segredos inacessível: ${(e as Error).message}`,
    }
  }

  try {
    const at = new Date().toISOString()
    await writeSecretData(ORG_HEALTH_SECRET_REF, {
      ping: 'ok',
      at,
      source: 'archgate-manager-health',
    })
    const back = await readSecretData(ORG_HEALTH_SECRET_REF)
    if (!back?.ping) {
      return {
        write_ok: false,
        reachable,
        sealed,
        detail: 'Write ok mas leitura do canário falhou (policy de leitura).',
      }
    }
    return { write_ok: true, reachable, sealed }
  } catch (e) {
    return {
      write_ok: false,
      reachable,
      sealed,
      detail: friendlySecretBackendError((e as Error).message),
    }
  }
}

/** Map raw OpenBao errors to Manager-facing Portuguese (no "abra o OpenBao"). */
export function friendlySecretBackendError(raw: string): string {
  const m = raw.toLowerCase()
  if (m.includes('permission denied') || m.includes('403')) {
    return 'Sem permissão para gravar no backend de segredos. Contate admin de plataforma (policy do app token).'
  }
  if (m.includes('404') || m.includes('invalid path') || m.includes('no handler')) {
    return 'Path de segredo org indisponível no backend. Contate admin de plataforma (mount secret/org).'
  }
  if (m.includes('sealed')) {
    return 'Backend de segredos selado. Contate admin de plataforma.'
  }
  if (m.includes('ausente') || m.includes('token')) {
    return 'Backend de segredos sem credencial no console. Contate admin de plataforma.'
  }
  if (m.includes('econnrefused') || m.includes('fetch failed') || m.includes('enotfound')) {
    return 'Backend de segredos inacessível. Contate admin de plataforma.'
  }
  // Keep short technical tail for support
  const short = raw.length > 160 ? `${raw.slice(0, 160)}…` : raw
  return `Falha no backend de segredos: ${short}`
}

export async function getOrgBrokerHealth(): Promise<OrgBrokerHealth> {
  const settings = getOrgBrokerSettings()
  const probe = await probeOrgSecretBackend()
  const accounts = listOrgAccounts()
  const withRef = accounts.filter((a) => !!a.secret_ref?.trim()).length
  const p0Missing = accounts.filter(
    (a) => a.criticality === 'P0' && !a.secret_ref?.trim(),
  ).length

  const hints: string[] = []
  if (!probe.write_ok) {
    hints.push(
      'Checkout e “Gravar secret” vão falhar até o backend de segredos ficar OK. Isso se resolve no bootstrap da plataforma — não é necessário UI externa no dia a dia.',
    )
  }
  if (!settings.checkout_webhook_url) {
    hints.push(
      'Webhook de dual-control não configurado. Em Contas da org → Configurações do broker, defina uma URL HTTPS (Slack ou genérica).',
    )
  }
  if (p0Missing > 0) {
    hints.push(
      `${p0Missing} conta(s) P0 sem secret_ref — use Editar ou “Gravar secret” (o path é criado automaticamente).`,
    )
  }
  if (probe.write_ok && !hints.length) {
    hints.push(
      'Broker operacional: inventário, checkout e secrets via este console (estilo AWS Console).',
    )
  }

  const ok =
    probe.write_ok &&
    openbaoTokenConfigured() &&
    (probe.sealed === false || probe.sealed === null)

  return {
    ok: !!ok && probe.write_ok,
    openbao: {
      address_configured: openbaoConfigured(),
      token_configured: openbaoTokenConfigured(),
      token_kind: openbaoTokenKind(),
      reachable: probe.reachable,
      sealed: probe.sealed,
      write_ok: probe.write_ok,
      detail: probe.detail,
    },
    settings: {
      webhook_configured: !!settings.checkout_webhook_url,
      ttl_default_seconds: settings.ttl_default_seconds,
      ttl_max_seconds: settings.ttl_max_seconds,
    },
    accounts: {
      total: accounts.length,
      with_secret_ref: withRef,
      p0_missing_secret_ref: p0Missing,
    },
    hints,
  }
}
