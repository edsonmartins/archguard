// Resolve operator session for Connect BFF: cookie, Bearer OIDC, or lab token.

import { timingSafeEqual } from 'node:crypto'
import type { SessionData } from './auth'
import { requireUnifiedSession } from './unified-bff'
import { logger } from './logger'

/**
 * Lab bearer (smokes / offline dev). Fail-closed by design:
 *
 * - `ARCHGATE_CONNECT_LAB=1` alone is NOT enough — the shared secret in
 *   `ARCHGATE_CONNECT_LAB_TOKEN` is mandatory, so a bare `Bearer lab-<user>`
 *   from the internet can never mint a session (audit 2026-07-28, P0-1).
 * - The lab principal never inherits a real client tenant. Groups come from
 *   `ARCHGATE_CONNECT_LAB_GROUPS` and default to `archguard_users` only.
 *
 * Token format: `lab-<username>:<ARCHGATE_CONNECT_LAB_TOKEN>`
 */
const LAB_GROUPS_DEFAULT = 'archguard_users'

function labEnabled(): boolean {
  return process.env.ARCHGATE_CONNECT_LAB === '1'
}

function labSecret(): string {
  return process.env.ARCHGATE_CONNECT_LAB_TOKEN || ''
}

let labMisconfigLogged = false

function secretMatches(candidate: string): boolean {
  const expected = labSecret()
  if (!expected || !candidate) return false
  const a = Buffer.from(candidate)
  const b = Buffer.from(expected)
  if (a.length !== b.length) return false
  return timingSafeEqual(a, b)
}

/** Returns a lab session only for a well-formed, correctly signed lab bearer. */
export function labSession(token: string): SessionData | null {
  if (!labEnabled() || !token.startsWith('lab-')) return null
  if (!labSecret()) {
    if (!labMisconfigLogged) {
      labMisconfigLogged = true
      logger.error(
        'ARCHGATE_CONNECT_LAB=1 without ARCHGATE_CONNECT_LAB_TOKEN — lab bearer refused',
      )
    }
    return null
  }
  const sep = token.indexOf(':')
  if (sep < 0) return null
  const username = token.slice(4, sep)
  if (!username || !secretMatches(token.slice(sep + 1))) return null

  const groups = (process.env.ARCHGATE_CONNECT_LAB_GROUPS || LAB_GROUPS_DEFAULT)
    .split(',')
    .map((g) => g.trim())
    .filter(Boolean)

  return {
    isAuthenticated: true,
    isAdmin: false,
    user: {
      id: `lab:${username}`,
      name: username,
      email: `${username}@lab.local`,
      displayName: username,
    },
    groups,
    permissions: [],
    expiresAt: Date.now() + 8 * 60 * 60 * 1000,
  } as SessionData
}

/**
 * Session for Connect CLI / desktop:
 * 1) cookie archguard_session (browser / test-login)
 * 2) Bearer OIDC access token → Kanidm userinfo
 * 3) Bearer lab-<user>:<secret> when the lab path is explicitly configured
 */
export async function resolveOperatorSession(
  request: Request,
): Promise<SessionData> {
  try {
    return requireUnifiedSession()
  } catch {
    /* try bearer */
  }
  const auth = request.headers.get('Authorization') || ''
  const m = auth.match(/^Bearer\s+(.+)$/i)
  if (!m) throw new Error('Unauthorized')

  const token = m[1]
  const lab = labSession(token)
  if (lab) return lab
  // A `lab-` bearer never falls through to the IdP — it is not an OIDC token.
  if (token.startsWith('lab-')) throw new Error('Unauthorized')

  const issuer =
    process.env.CONNECT_OIDC_ISSUER ||
    process.env.OIDC_ISSUER_BASE ||
    'https://id.archgate.com.br/oauth2/openid/archgate-connect'
  const userinfoURL = `${issuer.replace(/\/$/, '')}/userinfo`
  const res = await fetch(userinfoURL, {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!res.ok) {
    throw new Error('Unauthorized')
  }
  const ui = (await res.json()) as {
    sub?: string
    preferred_username?: string
    name?: string
    email?: string
    groups?: string[]
  }
  const username = ui.preferred_username || ui.name || ui.email || 'operator'
  const groups = Array.isArray(ui.groups) ? ui.groups : []
  return {
    isAuthenticated: true,
    isAdmin: groups.some((g) => String(g).includes('archguard_super_admins')),
    user: {
      id: ui.sub || username,
      name: username,
      email: ui.email || '',
      displayName: ui.name || username,
    },
    groups: groups.map(String),
    permissions: [],
    expiresAt: Date.now() + 60 * 60 * 1000,
  } as SessionData
}
