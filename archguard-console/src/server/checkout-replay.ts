import type { IdempotencyHit } from './bff-idempotency'

/** Never fall back to the original checkout response from a legacy cache. */
export function checkoutReplay(hit: IdempotencyHit): { status: number; body: unknown } {
  if (!hit.completed) return { status: 409, body: { error: 'request already in progress' } }
  const body = hit.replayResponse
  if (!hit.replayStatusCode || !body || typeof body !== 'object' || 'secret' in body) {
    return { status: 409, body: { error: 'checkout replay unavailable; consult checkout status' } }
  }
  return { status: hit.replayStatusCode, body }
}
