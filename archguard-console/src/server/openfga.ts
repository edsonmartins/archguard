import { integrationFetch } from './http-integration-client'

type CheckResponse = { allowed?: boolean }

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
