// src/server/auth.ts

import { createServerFn } from '@tanstack/react-start'
import {
  getCookie,
  setCookie,
  deleteCookie,
} from '@tanstack/react-start/server'
import { encryptSession, decryptSession } from './session'
import { derivePermissions, ALL_PERMISSIONS, type Permission } from '../lib/auth/permissions'

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
}

const KANIDM_URL = process.env.ARCHGUARD_ID_URL || 'https://localhost:8443'
const OIDC_CLIENT_ID = 'archguard-console'

const COOKIE_OPTIONS = {
  httpOnly: true,
  secure: process.env.NODE_ENV === 'production',
  sameSite: 'lax' as const,
  path: '/',
  maxAge: 86400,
}

async function exchangeCodeForTokens(
  code: string,
  codeVerifier: string,
  redirectUri: string,
) {
  const tokenUrl = `${KANIDM_URL}/oauth2/token`
  console.log(`[auth] Token exchange: POST ${tokenUrl}`)
  console.log(`[auth] redirect_uri=${redirectUri}, code_verifier length=${codeVerifier.length}`)

  const response = await fetch(tokenUrl, {
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
    const error = await response.text()
    console.error(`[auth] Token exchange failed (${response.status}): ${error}`)
    throw new Error(`Token exchange failed (${response.status}): ${error}`)
  }

  const tokens = await response.json() as {
    access_token: string
    id_token: string
    refresh_token?: string
    expires_in: number
    token_type: string
  }
  console.log(`[auth] Token exchange OK, expires_in=${tokens.expires_in}`)
  return tokens
}

function decodeJWT(token: string): Record<string, unknown> {
  const parts = token.split('.')
  if (parts.length !== 3) throw new Error('Invalid JWT')
  const payload = Buffer.from(parts[1]!, 'base64url').toString('utf8')
  return JSON.parse(payload)
}

export const getSessionFn = createServerFn({ method: 'GET' }).handler(
  async (): Promise<SessionData | null> => {
    const sessionCookie = getCookie('archguard_session')
    if (!sessionCookie) {
      console.log('[auth] getSession - no cookie found')
      return null
    }
    console.log(`[auth] getSession - cookie found, size=${sessionCookie.length}`)

    try {
      const session = decryptSession<SessionData>(sessionCookie)
      console.log(`[auth] getSession - decrypted OK, user=${session.user?.name}`)
      return session
    } catch (err) {
      console.error('[auth] getSession - decrypt failed:', err)
      deleteCookie('archguard_session', { path: '/' })
      return null
    }
  },
)

export const loginCallbackFn = createServerFn({ method: 'POST' })
  .inputValidator(
    (data: {
      code: string
      state: string
      codeVerifier: string
      redirectUri: string
    }) => data,
  )
  .handler(async ({ data }) => {
    try {
      // 1. Exchange authorization code for tokens (server-side only)
      const tokens = await exchangeCodeForTokens(
        data.code,
        data.codeVerifier,
        data.redirectUri,
      )

      // 2. Decode id_token to extract claims
      // Note: In production, verify JWT signature against Kanidm JWKS endpoint
      const claims = decodeJWT(tokens.id_token)
      console.log('[auth] id_token claims:', JSON.stringify(Object.keys(claims)))

      // Groups may be in 'groups' claim (Kanidm with groups scope)
      const rawGroups: string[] = (claims.groups as string[]) || []
      const groups = normalizeGroups(rawGroups)
      console.log('[auth] User groups (normalized):', groups)

      // 3. Check if user has admin/service desk access
      const adminGroups = [
        'archguard_super_admins',
        'archguard_tenant_admins',
        'archguard_admins',
        'idm_admins',
        'idm_people_admins',
        'idm_oauth2_admins',
      ]
      const isAdmin = groups.some(
        (g) => adminGroups.includes(g) || g.endsWith('_admins'),
      )

      const accessGroups = [
        'archguard_users',
        'archguard_service_desk',
        'archguard_viewers',
        'idm_service_desk',
      ]
      const hasAccess =
        isAdmin || groups.some((g) => accessGroups.includes(g))

      // If no groups were returned (scope not mapped yet), allow access for any authenticated user
      const allowNoGroups = groups.length === 0
      if (!hasAccess && !allowNoGroups) {
        console.log('[auth] Access denied - groups:', groups)
        return { success: false as const, error: 'unauthorized', redirect: '/unauthorized' }
      }

      if (allowNoGroups) {
        console.log('[auth] Warning: no groups in token, allowing access as fallback')
      }

      // 4. Derive permissions from groups
      const permissions = derivePermissions(groups)

      // 5. Create session (SA token NOT stored in session - used server-side only)
      const session: SessionData = {
        isAuthenticated: true,
        isAdmin: isAdmin || allowNoGroups,
        user: {
          id: claims.sub as string,
          name: (claims.preferred_username as string) || (claims.name as string) || 'unknown',
          email: (claims.email as string) || '',
          displayName: (claims.name as string) || (claims.preferred_username as string) || 'User',
        },
        groups,
        permissions: allowNoGroups ? ALL_PERMISSIONS : permissions,
        expiresAt: Date.now() + tokens.expires_in * 1000,
      }

      const encrypted = encryptSession(session)
      console.log(`[auth] Session encrypted, size=${encrypted.length} bytes`)

      setCookie(
        'archguard_session',
        encrypted,
        COOKIE_OPTIONS,
      )

      return { success: true as const, redirect: '/dashboard' }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      console.error('[auth] loginCallbackFn error:', message)
      return { success: false as const, error: message }
    }
  })

// Normalize Kanidm group names: strip @domain suffix and filter out UUIDs
function normalizeGroups(rawGroups: string[]): string[] {
  return rawGroups
    .filter((g) => !g.match(/^[0-9a-f]{8}-[0-9a-f]{4}-/)) // Remove UUIDs
    .map((g) => g.replace(/@.*$/, '')) // Strip @domain suffix
}

// Session creation from client-side token exchange (used by signinCallback flow)
export const createSessionFromTokensFn = createServerFn({ method: 'POST' })
  .inputValidator(
    (data: {
      accessToken: string
      idToken: string
      refreshToken?: string
      expiresIn: number
    }) => data,
  )
  .handler(async ({ data }) => {
    try {
      const claims = decodeJWT(data.idToken)
      console.log('[auth] createSession - id_token claims:', JSON.stringify(Object.keys(claims)))

      const rawGroups: string[] = (claims.groups as string[]) || []
      const groups = normalizeGroups(rawGroups)
      console.log('[auth] createSession - normalized groups:', groups)

      const adminGroups = [
        'archguard_super_admins',
        'archguard_tenant_admins',
        'archguard_admins',
        'idm_admins',
        'idm_people_admins',
        'idm_oauth2_admins',
      ]
      const isAdmin = groups.some(
        (g) => adminGroups.includes(g) || g.endsWith('_admins'),
      )

      const accessGroups = [
        'archguard_users',
        'archguard_service_desk',
        'archguard_viewers',
        'idm_service_desk',
      ]
      const hasAccess =
        isAdmin || groups.some((g) => accessGroups.includes(g))

      if (!hasAccess) {
        console.log('[auth] createSession - access denied, groups:', groups)
        return { success: false as const, error: 'unauthorized', redirect: '/unauthorized' }
      }

      const permissions = derivePermissions(groups)

      const session: SessionData = {
        isAuthenticated: true,
        isAdmin,
        user: {
          id: (claims.sub as string) || 'unknown',
          name: (claims.preferred_username as string) || (claims.name as string) || 'unknown',
          email: (claims.email as string) || '',
          displayName: (claims.name as string) || (claims.preferred_username as string) || 'User',
        },
        groups,
        permissions,
        expiresAt: Date.now() + data.expiresIn * 1000,
      }

      const encrypted = encryptSession(session)
      console.log(`[auth] createSession - OK, cookie size=${encrypted.length}`)

      setCookie('archguard_session', encrypted, COOKIE_OPTIONS)

      return { success: true as const, redirect: '/dashboard' }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      console.error('[auth] createSessionFromTokensFn error:', message)
      return { success: false as const, error: message }
    }
  })

export const logoutFn = createServerFn({ method: 'POST' }).handler(
  async () => {
    deleteCookie('archguard_session', { path: '/' })
    return { success: true }
  },
)
