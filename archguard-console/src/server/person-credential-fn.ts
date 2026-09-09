import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import { assertPrincipalTenantAccess, requireAnyPerm, requireSession, sessionActor } from './session-guard'
import { integrationFetch } from './http-integration-client'
import { enforceRateLimit } from './rate-limit'
import { recordActivity } from './activity-log'

const identifier = z.string().min(1).max(256).refine((s) =>
  s.trim() === s && s !== '.' && s !== '..' && !/[\\/\x00-\x1f\x7f]/.test(s),
)
const inputSchema = z.object({ id: identifier, ttl: z.number().int().min(60).max(604800).default(3600) })
const personSchema = z.object({ attrs: z.object({
  uuid: z.tuple([identifier]),
  name: z.tuple([identifier]),
}) })

export const resetPersonCredentialFn = createServerFn({ method: 'POST' })
  .inputValidator((input: unknown) => inputSchema.parse(input))
  .handler(async ({ data }) => {
    const session = requireSession()
    requireAnyPerm(session, ['persons:credentials'])
    enforceRateLimit('person-credential-reset', 10, 60_000)
    const base = (process.env.ARCHGUARD_ID_URL || '').replace(/\/$/, '')
    const token = process.env.ARCHGUARD_SA_TOKEN
    if (!base || !token) throw new Error('Identity integration is not configured')
    const request = async (path: string, method: string) => {
      const response = await integrationFetch(`${base}${path}`, {
        method, redirect: 'error', integration: 'person-credential-reset',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      })
      if (!response.ok) throw new Error('Identity credential operation failed')
      try { return await response.json() as unknown } catch {
        throw new Error('Invalid identity response')
      }
    }
    const auditPath = `/archgate/persons/${encodeURIComponent(data.id)}/credential-reset`
    try {
      // Resolve route UUID/name on the server. Never accept a client-supplied
      // username as proof of ownership of a different route ID.
      const parsed = personSchema.safeParse(await request(`/v1/person/${encodeURIComponent(data.id)}`, 'GET'))
      if (!parsed.success) throw new Error('Invalid identity response')
      const { uuid: [uuid], name: [username] } = parsed.data.attrs
      if (data.id !== uuid && data.id !== username) throw new Error('Identity mismatch')
      await assertPrincipalTenantAccess(username, session)
      const reset = z.object({ token: z.string().min(1) }).safeParse(
        await request(`/v1/person/${encodeURIComponent(uuid)}/_credential/_update_intent/${data.ttl}`, 'POST'),
      )
      if (!reset.success) throw new Error('Invalid credential reset response')
      recordActivity('POST', auditPath, sessionActor(session), 'success')
      return reset.data
    } catch (error) {
      // Reset tokens and upstream response bodies must never enter the audit.
      recordActivity('POST', auditPath, sessionActor(session), 'error', 'Credential reset failed')
      throw error
    }
  })
