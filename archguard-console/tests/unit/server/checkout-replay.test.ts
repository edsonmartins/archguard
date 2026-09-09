import { expect, it } from 'vitest'
import { checkoutReplay } from '@/server/checkout-replay'
import type { IdempotencyHit } from '@/server/bff-idempotency'
const legacy: IdempotencyHit = {
  bodyHash: 'hash', statusCode: 200, completed: true,
  response: { secret: { password: 'synthetic-secret' } },
  replayResponse: null, replayStatusCode: null,
}
it('denies the original legacy response without re-exposing its secret', () => {
  const result = checkoutReplay(legacy)
  expect(result.status).toBe(409)
  expect(JSON.stringify(result)).not.toContain('synthetic-secret')
})
it('also rejects secrets accidentally persisted in the replay column', () => {
  expect(checkoutReplay({ ...legacy, replayStatusCode: 200, replayResponse: legacy.response }).status).toBe(409)
})
it('retains safe pending approval metadata without returning original data', () => {
  const safe = { checkout: { id: 'one', status: 'pending' }, message: 'pending dual-control' }
  expect(checkoutReplay({ ...legacy, replayStatusCode: 200, replayResponse: safe })).toEqual({ status: 200, body: safe })
})
