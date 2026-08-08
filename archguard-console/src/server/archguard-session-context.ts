import { integrationFetch } from './http-integration-client'

export type ArchGuardMembership = {
  membership_id: string
  organization_id: string
  status: string
}

export type ArchGuardSessionContext = {
  subject: string
  identity_id: string
  identity_status: string
  memberships: ArchGuardMembership[]
}

/** Resolve the authenticated OIDC subject through the internal control plane. */
export async function resolveArchGuardSessionContext(
  subject: string,
): Promise<ArchGuardSessionContext> {
  const base = (process.env.ORCHESTRATION_URL || '').replace(/\/$/, '')
  const token = process.env.ORCH_API_TOKEN || ''
  if (!base || !token) throw new Error('ArchGuard session context is not configured')
  const res = await integrationFetch(`${base}/orchestration/v1/identities/session-context`, {
    method: 'POST',
    integration: 'archguard-session-context',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ subject }),
  })
  const text = await res.text()
  if (!res.ok) throw new Error(`ArchGuard session context failed: ${res.status}`)
  const context = JSON.parse(text) as ArchGuardSessionContext
  if (!context.subject || context.subject !== subject) {
    throw new Error('ArchGuard session context subject mismatch')
  }
  if (context.identity_status !== 'active') {
    throw new Error('ArchGuard identity is not active')
  }
  return {
    ...context,
    memberships: Array.isArray(context.memberships) ? context.memberships : [],
  }
}
