// Resolve operator session for Connect BFF: cookie, Bearer OIDC, or lab token.

import type { SessionData } from './auth'
import { requireUnifiedSession } from './unified-bff'

/**
 * Session for Connect CLI / desktop:
 * 1) cookie archguard_session (browser / test-login)
 * 2) Bearer OIDC access token → Kanidm userinfo
 * 3) Bearer lab-* when ARCHGATE_CONNECT_LAB=1
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
  if (process.env.ARCHGATE_CONNECT_LAB === '1' && token.startsWith('lab-')) {
    const username = token.slice(4) || 'lab.operator'
    return {
      isAuthenticated: true,
      isAdmin: false,
      user: {
        id: `lab:${username}`,
        name: username,
        email: `${username}@lab.local`,
        displayName: username,
      },
      groups: ['archguard_users', 'tenant_rio_quality'],
      permissions: [],
      expiresAt: Date.now() + 8 * 60 * 60 * 1000,
    } as SessionData
  }

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
