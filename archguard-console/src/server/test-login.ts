// src/server/test-login.ts
//
// PROGRAMMATIC LOGIN FOR E2E ONLY.
//
// The OIDC redirect flow is unreliable in Playwright when Kanidm uses a
// self-signed cert (`oidc-client-ts` does its own discovery fetch and the
// browser refuses the response intermittently). This server function lets
// the E2E suite log in by username + password against Kanidm's `/v1/auth`
// API and mint a console session directly, skipping the browser redirect.
//
// Hardened: the `ARCHGUARD_E2E_LOGIN` env var must be set to "1" at server
// startup. The env var is only ever set by the E2E harness; production
// deployments leave it unset, so the handler short-circuits with an error.
// (We dropped the NODE_ENV check because Vite inlines NODE_ENV=production in
// the bundled output and we still want this route to work when the suite
// runs against a `vite build` artifact.)

import { createServerFn } from '@tanstack/react-start'
import { setCookie } from '@tanstack/react-start/server'
import { z } from 'zod'
import { encryptSession } from './session'
import { logger } from './logger'
import { derivePermissions, type Permission } from '../lib/auth/permissions'
import type { SessionData } from './auth'

const KANIDM_URL = process.env.ARCHGUARD_ID_URL || 'https://localhost:8443'
const KANIDM_SA_TOKEN = process.env.ARCHGUARD_SA_TOKEN!
const E2E_ENABLED = process.env.ARCHGUARD_E2E_LOGIN === '1'

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

const inputSchema = z.object({
  username: z.string().min(1).max(256),
  password: z.string().min(1).max(512),
})

function normalizeGroups(raw: string[]): string[] {
  return raw
    .filter((g) => !g.match(/^[0-9a-f]{8}-[0-9a-f]{4}-/))
    .map((g) => g.replace(/@.*$/, ''))
}

interface KanidmAuthState {
  // Kanidm v1.9 returns the session token directly as a string in
  // `state.success`, not as an object.
  state?: { choose?: string[]; continue?: string[]; success?: string }
}

async function kanidmAuth(
  username: string,
  password: string,
): Promise<string> {
  // Step 1: init
  const init = await fetch(`${KANIDM_URL}/v1/auth`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ step: { init: username } }),
  })
  if (!init.ok) throw new Error(`Kanidm /v1/auth init: ${init.status}`)
  const sessionId = init.headers.get('x-kanidm-auth-session-id') ?? ''
  if (!sessionId) throw new Error('Missing x-kanidm-auth-session-id header')

  // Step 2: begin (choose method)
  const beginRes = await fetch(`${KANIDM_URL}/v1/auth`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'x-kanidm-auth-session-id': sessionId,
    },
    body: JSON.stringify({ step: { begin: 'password' } }),
  })
  if (!beginRes.ok) throw new Error(`Kanidm /v1/auth begin: ${beginRes.status}`)

  // Step 3: cred (submit password)
  const credRes = await fetch(`${KANIDM_URL}/v1/auth`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'x-kanidm-auth-session-id': sessionId,
    },
    body: JSON.stringify({ step: { cred: { password } } }),
  })
  if (!credRes.ok) throw new Error(`Kanidm /v1/auth cred: ${credRes.status}`)
  const credData = (await credRes.json()) as KanidmAuthState
  const token = credData.state?.success
  if (!token || typeof token !== 'string') {
    throw new Error('Kanidm auth did not return a session token')
  }
  return token
}

interface KanidmEntry {
  attrs?: { uuid?: string[]; name?: string[]; displayname?: string[]; mail?: string[]; memberof?: string[] }
}

async function fetchPerson(username: string): Promise<KanidmEntry> {
  const res = await fetch(
    `${KANIDM_URL}/v1/person/${encodeURIComponent(username)}`,
    {
      headers: { Authorization: `Bearer ${KANIDM_SA_TOKEN}` },
    },
  )
  if (!res.ok) throw new Error(`Kanidm /v1/person: ${res.status}`)
  return (await res.json()) as KanidmEntry
}

export const testLoginFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = inputSchema.safeParse(data)
    if (!r.success) throw new Error(`Invalid input: ${r.error.message}`)
    return r.data
  })
  .handler(async ({ data }) => {
    if (!E2E_ENABLED) {
      throw new Error(
        'test-login is disabled. Set ARCHGUARD_E2E_LOGIN=1 to enable.',
      )
    }

    // Verify the password is correct against Kanidm.
    await kanidmAuth(data.username, data.password)

    // Pull groups via the service-account token so we know what to grant.
    const person = await fetchPerson(data.username)
    const groups = normalizeGroups(person.attrs?.memberof ?? [])

    const isAdmin = groups.some(
      (g) => ADMIN_GROUPS.includes(g) || g.endsWith('_admins'),
    )
    const hasAccess = isAdmin || groups.some((g) => ACCESS_GROUPS.includes(g))
    if (!hasAccess) {
      logger.warn(
        { username: data.username, groups },
        'test-login: user has no access',
      )
      return { success: false as const, error: 'unauthorized' }
    }

    const session: SessionData = {
      isAuthenticated: true,
      isAdmin,
      user: {
        id: person.attrs?.uuid?.[0] ?? data.username,
        name: data.username,
        email: person.attrs?.mail?.[0] ?? '',
        displayName: person.attrs?.displayname?.[0] ?? data.username,
      },
      groups,
      permissions: derivePermissions(groups) as Permission[],
      expiresAt: Date.now() + 86_400_000,
    }

    setCookie('archguard_session', encryptSession(session), {
      httpOnly: true,
      secure: false, // dev/test only; never reached in prod
      sameSite: 'lax',
      path: '/',
      maxAge: 86_400,
    })

    return { success: true as const }
  })
