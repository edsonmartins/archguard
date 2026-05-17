// src/server/auth.ts

import { createServerFn } from '@tanstack/react-start'
import {
  getCookie,
  setCookie,
  deleteCookie,
} from '@tanstack/react-start/server'
import { z } from 'zod'
import { encryptSession, decryptSession } from './session'
import { verifyIdToken } from './jwt'
import { logger } from './logger'
import { enforceRateLimit } from './rate-limit'
import { derivePermissions, type Permission } from '../lib/auth/permissions'

const LOGIN_LIMIT = 10
const LOGIN_WINDOW_MS = 60 * 1000

export interface SessionUser {
  id: string
  name: string
  email: string
  displayName: string
}

export interface SessionData {
  isAuthenticated: boolean
  isAdmin: boolean
  user: SessionUser
  groups: string[]
  permissions: Permission[]
  expiresAt: number
  refreshToken?: string
}

// Refresh proactively when fewer than this many ms remain on the access token.
const REFRESH_THRESHOLD_MS = 60 * 1000

const KANIDM_URL = process.env.ARCHGUARD_ID_URL || 'https://localhost:8443'
const OIDC_CLIENT_ID = 'archguard-console'
const IS_PROD = process.env.NODE_ENV === 'production'

if (IS_PROD && !KANIDM_URL.startsWith('https://')) {
  throw new Error(
    'ARCHGUARD_ID_URL must be HTTPS in production (got: ' + KANIDM_URL + ')',
  )
}

const COOKIE_OPTIONS = {
  httpOnly: true,
  secure: IS_PROD,
  sameSite: 'lax' as const,
  path: '/',
  maxAge: 86400,
}

const ADMIN_GROUPS = [
  'archguard_super_admins',
  'archguard_tenant_admins',
  'archguard_admins',
  'idm_admins',
  'idm_people_admins',
  'idm_oauth2_admins',
]

const ACCESS_GROUPS = [
  'archguard_users',
  'archguard_service_desk',
  'archguard_viewers',
  'idm_service_desk',
]

const loginCallbackSchema = z.object({
  code: z.string().min(1).max(2048),
  state: z.string().min(1).max(512),
  codeVerifier: z.string().min(43).max(128),
  redirectUri: z.string().url().max(2048),
})

const sessionFromTokensSchema = z.object({
  accessToken: z.string().min(1).max(8192),
  idToken: z.string().min(1).max(8192),
  refreshToken: z.string().max(8192).optional(),
  expiresIn: z.number().int().positive().max(86400),
})

function parseInput<T extends z.ZodTypeAny>(
  schema: T,
  data: unknown,
): z.infer<T> {
  const result = schema.safeParse(data)
  if (!result.success) {
    throw new Error(`Invalid input: ${result.error.message}`)
  }
  return result.data
}

function evaluateAccess(groups: string[]) {
  const isAdmin = groups.some(
    (g) => ADMIN_GROUPS.includes(g) || g.endsWith('_admins'),
  )
  const hasAccess = isAdmin || groups.some((g) => ACCESS_GROUPS.includes(g))
  return { isAdmin, hasAccess }
}

async function exchangeCodeForTokens(
  code: string,
  codeVerifier: string,
  redirectUri: string,
) {
  const response = await fetch(`${KANIDM_URL}/oauth2/token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({
      grant_type: 'authorization_code',
      code,
      redirect_uri: redirectUri,
      client_id: OIDC_CLIENT_ID,
      code_verifier: codeVerifier,
    }),
  })

  if (!response.ok) {
    throw new Error(`Token exchange failed (${response.status})`)
  }

  return (await response.json()) as TokenResponse
}

interface TokenResponse {
  access_token: string
  id_token: string
  refresh_token?: string
  expires_in: number
  token_type: string
}

async function exchangeRefreshTokenForTokens(
  refreshToken: string,
): Promise<TokenResponse> {
  const response = await fetch(`${KANIDM_URL}/oauth2/token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({
      grant_type: 'refresh_token',
      refresh_token: refreshToken,
      client_id: OIDC_CLIENT_ID,
    }),
  })

  if (!response.ok) {
    throw new Error(`Token refresh failed (${response.status})`)
  }
  return (await response.json()) as TokenResponse
}

/**
 * Build a SessionData payload from a Kanidm token response. Verifies the
 * id_token and re-derives groups/permissions, so any group changes upstream
 * propagate on every refresh.
 */
async function sessionFromTokens(
  tokens: TokenResponse,
): Promise<SessionData | null> {
  const claims = await verifyIdToken(tokens.id_token)
  const rawGroups: string[] = (claims.groups as string[]) || []
  const groups = normalizeGroups(rawGroups)
  const { isAdmin, hasAccess } = evaluateAccess(groups)
  if (!hasAccess) return null

  return {
    isAuthenticated: true,
    isAdmin,
    user: {
      id: (claims.sub as string) || 'unknown',
      name:
        (claims.preferred_username as string) ||
        (claims.name as string) ||
        'unknown',
      email: (claims.email as string) || '',
      displayName:
        (claims.name as string) ||
        (claims.preferred_username as string) ||
        'User',
    },
    groups,
    permissions: derivePermissions(groups),
    expiresAt: Date.now() + tokens.expires_in * 1000,
    refreshToken: tokens.refresh_token,
  }
}

async function tryRefreshSession(
  current: SessionData,
): Promise<SessionData | null> {
  if (!current.refreshToken) return null
  try {
    const tokens = await exchangeRefreshTokenForTokens(current.refreshToken)
    return await sessionFromTokens(tokens)
  } catch (err) {
    logger.warn(
      { err: err instanceof Error ? err.message : String(err) },
      'auth: refresh failed',
    )
    return null
  }
}

export const getSessionFn = createServerFn({ method: 'GET' }).handler(
  async (): Promise<SessionData | null> => {
    const sessionCookie = getCookie('archguard_session')
    if (!sessionCookie) return null

    let session: SessionData
    try {
      session = decryptSession<SessionData>(sessionCookie)
    } catch {
      deleteCookie('archguard_session', { path: '/' })
      return null
    }

    const remaining = session.expiresAt - Date.now()
    if (remaining > REFRESH_THRESHOLD_MS) return session

    // Token is near expiry or already expired — try to refresh silently.
    const refreshed = await tryRefreshSession(session)
    if (!refreshed) {
      // No refresh token, refresh failed, or user lost access. Drop session.
      if (remaining <= 0) {
        deleteCookie('archguard_session', { path: '/' })
        return null
      }
      // Still valid for a short window — keep the existing one.
      return session
    }

    setCookie('archguard_session', encryptSession(refreshed), COOKIE_OPTIONS)
    return refreshed
  },
)

export const loginCallbackFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => parseInput(loginCallbackSchema, data))
  .handler(async ({ data }) => {
    try {
      enforceRateLimit('login', LOGIN_LIMIT, LOGIN_WINDOW_MS)

      const tokens = await exchangeCodeForTokens(
        data.code,
        data.codeVerifier,
        data.redirectUri,
      )

      const session = await sessionFromTokens(tokens)
      if (!session) {
        logger.warn('auth: access denied (no qualifying group)')
        return {
          success: false as const,
          error: 'unauthorized',
          redirect: '/unauthorized',
        }
      }

      setCookie('archguard_session', encryptSession(session), COOKIE_OPTIONS)
      return { success: true as const, redirect: '/dashboard' }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      logger.error({ err: message }, 'auth: login flow failed')
      return { success: false as const, error: message }
    }
  })

// Normalize Kanidm group names: strip @domain suffix and filter out UUIDs
export function normalizeGroups(rawGroups: string[]): string[] {
  return rawGroups
    .filter((g) => !g.match(/^[0-9a-f]{8}-[0-9a-f]{4}-/)) // Remove UUIDs
    .map((g) => g.replace(/@.*$/, '')) // Strip @domain suffix
}

// Session creation from client-side token exchange (used by signinCallback flow)
export const createSessionFromTokensFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => parseInput(sessionFromTokensSchema, data))
  .handler(async ({ data }) => {
    try {
      enforceRateLimit('login', LOGIN_LIMIT, LOGIN_WINDOW_MS)

      const session = await sessionFromTokens({
        access_token: data.accessToken,
        id_token: data.idToken,
        refresh_token: data.refreshToken,
        expires_in: data.expiresIn,
        token_type: 'Bearer',
      })

      if (!session) {
        logger.warn('auth: access denied (no qualifying group)')
        return {
          success: false as const,
          error: 'unauthorized',
          redirect: '/unauthorized',
        }
      }

      setCookie('archguard_session', encryptSession(session), COOKIE_OPTIONS)
      return { success: true as const, redirect: '/dashboard' }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      logger.error({ err: message }, 'auth: login flow failed')
      return { success: false as const, error: message }
    }
  })

export const logoutFn = createServerFn({ method: 'POST' }).handler(
  async () => {
    deleteCookie('archguard_session', { path: '/' })
    return { success: true }
  },
)
