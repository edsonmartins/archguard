import { integrationFetch } from './http-integration-client'

type CheckResponse = { allowed?: boolean }
type TupleKey = { user: string; relation: string; object: string }

function config() {
  return {
    enabled: process.env.OPENFGA_ENABLED === '1',
    url: (process.env.OPENFGA_URL || '').replace(/\/$/, ''),
    store: process.env.OPENFGA_STORE_ID || '',
    model: process.env.OPENFGA_MODEL_ID || '',
    token: process.env.OPENFGA_API_TOKEN || '',
  }
}

export function openFgaConfigured(): boolean {
  const c = config()
  return !c.enabled || Boolean(c.url && c.store && c.model && c.token)
}

export function openFgaEnabled(): boolean {
  return config().enabled
}

export function openFgaConnectionObject(siteSlug: string, target: string): string {
  return `connection:${siteSlug}/${target}`
}

/**
 * Check one relationship. OpenFGA is optional until its store is provisioned,
 * but enabling it without a complete config or an allowed decision fails closed.
 */
export async function checkOpenFga(input: {
  user: string
  relation: string
  object: string
  contextualTuples?: Array<{ user: string; relation: string; object: string }>
}): Promise<boolean> {
  const c = config()
  if (!c.enabled) return true
  if (!openFgaConfigured()) throw new Error('OpenFGA is not configured')
  const res = await integrationFetch(`${c.url}/stores/${encodeURIComponent(c.store)}/check`, {
    method: 'POST',
    integration: 'openfga',
    headers: {
      Authorization: `Bearer ${c.token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      authorization_model_id: c.model,
      tuple_key: {
        user: input.user,
        relation: input.relation,
        object: input.object,
      },
      ...(input.contextualTuples?.length
        ? { contextual_tuples: { tuple_keys: input.contextualTuples } }
        : {}),
    }),
  })
  if (!res.ok) throw new Error(`OpenFGA check failed: ${res.status}`)
  const body = (await res.json()) as CheckResponse
  if (body.allowed !== true) return false
  return true
}

/** Persist a direct grant after the downstream adapter accepted it. */
export async function writeOpenFgaGrant(input: {
  user: string
  relation: string
  object: string
}): Promise<void> {
  const c = config()
  if (!c.enabled) return
  if (!openFgaConfigured()) throw new Error('OpenFGA is not configured')
  const res = await integrationFetch(`${c.url}/stores/${encodeURIComponent(c.store)}/write`, {
    method: 'POST',
    integration: 'openfga',
    headers: { Authorization: `Bearer ${c.token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ writes: { tuple_keys: [input] } }),
  })
  if (!res.ok) throw new Error(`OpenFGA write failed: ${res.status}`)
}

/** Remove all direct grants currently materialized for a principal. */
export async function deleteOpenFgaGrantsForUser(user: string): Promise<number> {
  const c = config()
  if (!c.enabled) return 0
  if (!openFgaConfigured()) throw new Error('OpenFGA is not configured')
  if (!/^user:[^\s]+$/.test(user)) throw new Error('OpenFGA user subject invalid')
  const tuples: TupleKey[] = []
  let continuationToken: string | undefined
  const seenTokens = new Set<string>()
  for (let page = 0; page < 100; page += 1) {
    const read = await integrationFetch(
      `${c.url}/stores/${encodeURIComponent(c.store)}/read`,
      {
        method: 'POST',
        integration: 'openfga',
        headers: { Authorization: `Bearer ${c.token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({
          tuple_key: { user, relation: 'connect' },
          ...(continuationToken ? { continuation_token: continuationToken } : {}),
        }),
      },
    )
    if (!read.ok) throw new Error(`OpenFGA read failed: ${read.status}`)
    const body = (await read.json()) as {
      tuples?: Array<{ key?: Partial<TupleKey> }>
      continuation_token?: string
    }
    for (const tuple of body.tuples || []) {
      const key = tuple.key
      if (!key || key.user !== user || key.relation !== 'connect' ||
        typeof key.object !== 'string' || !key.object.startsWith('connection:')) {
        throw new Error('OpenFGA returned an invalid grant tuple')
      }
      tuples.push(key as TupleKey)
    }
    const next = body.continuation_token?.trim()
    if (!next) break
    if (seenTokens.has(next)) throw new Error('OpenFGA pagination token repeated')
    seenTokens.add(next)
    continuationToken = next
    if (page === 99) throw new Error('OpenFGA grant pagination exceeded limit')
  }
  let removed = 0
  for (let i = 0; i < tuples.length; i += 100) {
    const res = await integrationFetch(`${c.url}/stores/${encodeURIComponent(c.store)}/write`, {
      method: 'POST',
      integration: 'openfga',
      headers: { Authorization: `Bearer ${c.token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ deletes: { tuple_keys: tuples.slice(i, i + 100) } }),
    })
    if (!res.ok) throw new Error(`OpenFGA delete failed: ${res.status}`)
    removed += Math.min(100, tuples.length - i)
  }
  return removed
}
