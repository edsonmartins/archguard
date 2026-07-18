// POST /api/unified/v1/auth/callback — OIDC code exchange → session cookie
// Path: /api/unified/v1/auth/callback (file auth.callback → nested under auth if tree allows)
// Also registered as flat path via route id below.

import { createFileRoute } from '@tanstack/react-router'
import { setCookie } from '@tanstack/react-start/server'
import { exchangeCodeForTokens, sessionFromTokens } from '@/server/auth'
import { encryptSession } from '@/server/session'
import { profileFromSession } from '@/server/unified-bff'
import { logger } from '@/server/logger'

const COOKIE_OPTIONS = {
  path: '/',
  httpOnly: true,
  secure: process.env.NODE_ENV === 'production',
  sameSite: 'lax' as const,
  maxAge: 60 * 60 * 12,
}

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
    'Access-Control-Allow-Headers': 'Content-Type',
    'Access-Control-Allow-Methods': 'POST, OPTIONS',
  }
}

export const Route = createFileRoute('/api/unified/v1/auth/callback')({
  server: {
    handlers: {
      OPTIONS: async ({ request }) =>
        new Response(null, { status: 204, headers: corsHeaders(request) }),
      POST: async ({ request }) => {
        const headers = {
          'Content-Type': 'application/json',
          ...corsHeaders(request),
        }
        try {
          const body = (await request.json()) as {
            code?: string
            code_verifier?: string
            codeVerifier?: string
            redirect_uri?: string
            redirectUri?: string
          }
          const code = body.code
          const verifier = body.code_verifier || body.codeVerifier
          const redirectUri = body.redirect_uri || body.redirectUri
          if (!code || !verifier || !redirectUri) {
            return new Response(
              JSON.stringify({
                error: 'code, code_verifier and redirect_uri required',
              }),
              { status: 400, headers },
            )
          }
          const tokens = await exchangeCodeForTokens(
            code,
            verifier,
            redirectUri,
          )
          const session = await sessionFromTokens(tokens)
          if (!session) {
            return new Response(
              JSON.stringify({ error: 'unauthorized' }),
              { status: 403, headers },
            )
          }
          setCookie(
            'archguard_session',
            encryptSession(session),
            COOKIE_OPTIONS,
          )
          // Also set a readable name cookie for SPA display (non-secret)
          const profile = profileFromSession(session)
          return new Response(JSON.stringify(profile), { status: 200, headers })
        } catch (e) {
          const msg = (e as Error).message || 'callback failed'
          logger.error({ err: msg }, 'unified auth callback failed')
          return new Response(JSON.stringify({ error: msg }), {
            status: 500,
            headers,
          })
        }
      },
    },
  },
})
