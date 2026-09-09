import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import type { SessionData } from './auth'
import { assertPrincipalGroupsTenantAccess, hasAnyPerm, requireAnyPerm, requireSession } from './session-guard'
import { getUserGroups, normalizeGroupNames } from './idp'
import { deriveTenants } from '@/lib/auth/roles'
import { integrationFetch } from './http-integration-client'
import { enforceRateLimit } from './rate-limit'

const identifier = z.string().min(1).max(256).refine((s) =>
  s.trim() === s && s !== '.' && s !== '..' && !/[\\/\x00-\x1f\x7f]/.test(s),
)
// Explicit response projection: no credentials or unknown upstream attributes.
const values = z.array(z.string()).default([])
const personSchema = z.object({ attrs: z.object({
  uuid: z.tuple([identifier]), name: z.tuple([identifier]),
  displayname: values, legalname: values, mail: values, memberof: values,
  class: values, ssh_publickey: values, account_expire: values, account_valid_from: values,
}) })
type Entry = z.infer<typeof personSchema>

async function read(path: string): Promise<unknown> {
  const base = (process.env.ARCHGUARD_ID_URL || '').replace(/\/$/, '')
  const token = process.env.ARCHGUARD_SA_TOKEN
  if (!base || !token) throw new Error('Identity integration is not configured')
  const response = await integrationFetch(`${base}${path}`, {
    method: 'GET', redirect: 'error', integration: 'person-read',
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!response.ok) throw new Error('Identity query unavailable')
  try { return await response.json() as unknown } catch { throw new Error('Invalid identity response') }
}

function authorize(): SessionData {
  const session = requireSession()
  requireAnyPerm(session, ['persons:read'])
  if (!hasAnyPerm(session, ['system:admin']) && deriveTenants(session.groups).length === 0) {
    throw new Error('Forbidden: operador sem tenant')
  }
  enforceRateLimit('person-read', 30, 60_000)
  return session
}

async function scoped(entry: Entry, session: SessionData): Promise<Entry | null> {
  if (hasAnyPerm(session, ['system:admin'])) return entry
  const groups = await getUserGroups(entry.attrs.name[0])
  // An integration failure must not be presented as a successfully empty list.
  if (!groups) throw new Error('Identity membership verification unavailable')
  try { assertPrincipalGroupsTenantAccess(groups, session) } catch { return null }
  return { attrs: { ...entry.attrs, memberof: normalizeGroupNames(groups) } }
}

export const listPersonsFn = createServerFn({ method: 'GET' })
  .handler(async () => {
    const session = authorize()
    // Legacy upstream is unpaginated. Fail explicitly beyond the supported
    // inventory instead of returning an apparently complete truncated list.
    const parsed = z.array(personSchema).max(1000).safeParse(await read('/v1/person'))
    if (!parsed.success) throw new Error('Invalid or oversized identity inventory (maximum 1000)')
    const ids = new Set<string>()
    const names = new Set<string>()
    for (const entry of parsed.data) {
      if (ids.has(entry.attrs.uuid[0]) || names.has(entry.attrs.name[0])) throw new Error('Ambiguous identity inventory')
      ids.add(entry.attrs.uuid[0]); names.add(entry.attrs.name[0])
    }
    const result: Entry[] = []
    // Bound concurrent identity lookups; never return the global raw inventory.
    for (let i = 0; i < parsed.data.length; i += 8) {
      const batch = await Promise.all(parsed.data.slice(i, i + 8).map((entry) => scoped(entry, session)))
      result.push(...batch.filter((entry): entry is Entry => entry !== null))
    }
    return result
  })

export const getPersonFn = createServerFn({ method: 'GET' })
  .inputValidator((data: unknown) => z.object({ id: identifier }).parse(data))
  .handler(async ({ data }) => {
    const session = authorize()
    const parsed = personSchema.safeParse(await read(`/v1/person/${encodeURIComponent(data.id)}`))
    if (!parsed.success || (parsed.data.attrs.uuid[0] !== data.id && parsed.data.attrs.name[0] !== data.id)) {
      throw new Error('Identity unavailable')
    }
    const entry = await scoped(parsed.data, session)
    if (!entry) throw new Error('Identity unavailable')
    return entry
  })
