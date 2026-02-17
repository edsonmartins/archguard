// src/server/auth.ts

import { createServerFn } from '@tanstack/react-start'
import {
  getCookie,
  setCookie,
  deleteCookie,
} from '@tanstack/react-start/server'
import { encryptSession, decryptSession } from './session'
import { derivePermissions, type Permission } from '../lib/auth/permissions'

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
  oidcTokens: {
    accessToken: string
    idToken: string
    refreshToken?: string
    expiresAt: number
  }
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
    const error = await response.text()
    throw new Error(`Token exchange failed: ${error}`)
  }

  return response.json() as Promise<{
    access_token: string
    id_token: string
    refresh_token?: string
    expires_in: number
    token_type: string
  }>
}

function decodeJWT(token: string): Record<string, unknown> {
  const parts = token.split('.')
  if (parts.length !== 3) throw new Error('Invalid JWT')
  const payload = Buffer.from(parts[1]!, 'base64url').toString('utf8')
  return JSON.parse(payload)
}

async function refreshOIDCToken(refreshToken?: string) {
  if (!refreshToken) return null

  try {
    const response = await fetch(`${KANIDM_URL}/oauth2/token`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({
        grant_type: 'refresh_token',
        refresh_token: refreshToken,
        client_id: OIDC_CLIENT_ID,
      }),
    })

    if (!response.ok) return null

    const tokens = await response.json()
    return {
      accessToken: tokens.access_token,
      idToken: tokens.id_token,
      refreshToken: tokens.refresh_token,
      expiresAt: Date.now() + tokens.expires_in * 1000,
    }
  } catch {
    return null
  }
}

export const getSessionFn = createServerFn({ method: 'GET' }).handler(
  async (): Promise<SessionData | null> => {
    const sessionCookie = getCookie('archguard_session')
    if (!sessionCookie) return null

    try {
      const session = decryptSession<SessionData>(sessionCookie)

      // Check OIDC token expiration
      if (session.oidcTokens.expiresAt < Date.now()) {
        const refreshed = await refreshOIDCToken(
          session.oidcTokens.refreshToken,
        )
        if (!refreshed) {
          deleteCookie('archguard_session', { path: '/' })
          return null
        }
        session.oidcTokens = refreshed
        setCookie(
          'archguard_session',
          encryptSession(session),
          COOKIE_OPTIONS,
        )
      }

      return session
    } catch {
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
    // 1. Exchange authorization code for tokens (server-side only)
    const tokens = await exchangeCodeForTokens(
      data.code,
      data.codeVerifier,
      data.redirectUri,
    )

    // 2. Decode id_token to extract claims
    // Note: In production, verify JWT signature against Kanidm JWKS endpoint
    const claims = decodeJWT(tokens.id_token)
    const groups: string[] = (claims.groups as string[]) || []

    // 3. Check if user has admin/service desk access
    const adminGroups = [
      'archguard_admins',
      'idm_admins',
      'idm_people_admins',
      'idm_oauth2_admins',
    ]
    const isAdmin = groups.some(
      (g) => adminGroups.includes(g) || g.endsWith('_admins'),
    )

    const hasAccess =
      isAdmin || groups.includes('idm_service_desk')

    if (!hasAccess) {
      return { success: false as const, error: 'unauthorized', redirect: '/unauthorized' }
    }

    // 4. Derive permissions from groups
    const permissions = derivePermissions(groups)

    // 5. Create session (SA token NOT stored in session - used server-side only)
    const session: SessionData = {
      isAuthenticated: true,
      isAdmin,
      user: {
        id: claims.sub as string,
        name: (claims.preferred_username as string) || (claims.name as string),
        email: claims.email as string,
        displayName: claims.name as string,
      },
      groups,
      permissions,
      oidcTokens: {
        accessToken: tokens.access_token,
        idToken: tokens.id_token,
        refreshToken: tokens.refresh_token,
        expiresAt: Date.now() + tokens.expires_in * 1000,
      },
    }

    setCookie(
      'archguard_session',
      encryptSession(session),
      COOKIE_OPTIONS,
    )

    return { success: true as const, redirect: '/dashboard' }
  })

export const logoutFn = createServerFn({ method: 'POST' }).handler(
  async () => {
    deleteCookie('archguard_session', { path: '/' })
    return { success: true }
  },
)
