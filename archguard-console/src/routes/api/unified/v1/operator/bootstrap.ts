// GET /api/unified/v1/operator/bootstrap — ArchGate Connect + UnifiedUI
// Bastion endpoints + catalog (no customer private IPs). ADR-008A.

import { createFileRoute } from '@tanstack/react-router'
import {
  buildOperatorBootstrap,
  requireUnifiedSession,
} from '@/server/unified-bff'
import { logger } from '@/server/logger'
import type { SessionData } from '@/server/auth'

function corsHeaders(request: Request): HeadersInit {
  const origin = request.headers.get('Origin') || ''
  const allow =
    process.env.UNIFIED_UI_ORIGIN ||
    process.env.CORS_ALLOW_ORIGIN ||
    origin ||
    '*'
  return {
    'Access-Control-Allow-Origin': allow === '*' ? '*' : allow,
    'Access-Control-Allow-Credentials': 'true',
    'Access-Control-Allow-Headers':
      'Content-Type, Authorization, X-ArchGate-User, X-ArchGate-Tenants',
    'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
  }
}

/**
 * Session for Connect CLI:
 * 1) cookie archguard_session (browser)
 * 2) Bearer OIDC access token → Kanidm userinfo
 * 3) Bearer lab-* when ARCHGATE_CONNECT_LAB=1 (scaffold only)
 */
async function sessionForConnect(request: Request): Promise<SessionData> {
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

export const Route = createFileRoute('/api/unified/v1/operator/bootstrap')({
  server: {
    handlers: {
      OPTIONS: async ({ request }) =>
        new Response(null, { status: 204, headers: corsHeaders(request) }),
      GET: async ({ request }) => {
        const headers = {
          'Content-Type': 'application/json',
          ...corsHeaders(request),
        }
        try {
          const session = await sessionForConnect(request)
          const body = await buildOperatorBootstrap(session)
          return new Response(JSON.stringify(body), { status: 200, headers })
        } catch (e) {
          const msg = (e as Error).message || 'error'
          const status = msg.includes('Unauthorized') ? 401 : 500
          logger.warn({ err: msg }, 'operator bootstrap failed')
          return new Response(JSON.stringify({ error: msg }), {
            status,
            headers,
          })
        }
      },
    },
  },
})
