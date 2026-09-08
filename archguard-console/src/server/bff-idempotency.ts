import { createHash } from 'node:crypto'
import { getDb } from './db'

export type IdempotencyHit = {
  bodyHash: string
  statusCode: number | null
  response: unknown | null
  replayStatusCode: number | null
  replayResponse: unknown | null
  completed: boolean
}

export class IdempotencyConflict extends Error {
  constructor() {
    super('Idempotency-Key was already used with a different request body')
    this.name = 'IdempotencyConflict'
  }
}

export function hashBody(body: unknown): string {
  return createHash('sha256').update(JSON.stringify(body ?? null)).digest('hex')
}

/** Claim a key atomically. The scope must include organization, actor, method and path. */
export function claimIdempotency(scopeKey: string, bodyHash: string): IdempotencyHit | null {
  const db = getDb()
  const existing = db.prepare(
    'SELECT body_hash, status_code, response_json, replay_status_code, replay_response_json, completed_at FROM bff_idempotency WHERE scope_key = ?',
  ).get(scopeKey) as { body_hash: string; status_code: number | null; response_json: string | null; replay_status_code: number | null; replay_response_json: string | null; completed_at: string | null } | undefined

  if (existing) {
    if (existing.body_hash !== bodyHash) throw new IdempotencyConflict()
    return {
      bodyHash: existing.body_hash,
      statusCode: existing.status_code,
      response: existing.response_json ? JSON.parse(existing.response_json) : null,
      replayStatusCode: existing.replay_status_code,
      replayResponse: existing.replay_response_json ? JSON.parse(existing.replay_response_json) : null,
      completed: Boolean(existing.completed_at),
    }
  }

  try {
    db.prepare(
      `INSERT INTO bff_idempotency (scope_key, body_hash, created_at)
       VALUES (?, ?, ?)`,
    ).run(scopeKey, bodyHash, new Date().toISOString())
    return null
  } catch {
    // A concurrent request won the insert; read it and apply the same collision rules.
    return claimIdempotency(scopeKey, bodyHash)
  }
}

export function completeIdempotency(
  scopeKey: string,
  statusCode: number,
  response: unknown,
  replay?: { statusCode: number; response: unknown },
): void {
  getDb().prepare(
    `UPDATE bff_idempotency
        SET status_code = ?, response_json = ?, replay_status_code = ?, replay_response_json = ?, completed_at = ?
      WHERE scope_key = ?`,
  ).run(replay?.statusCode ?? statusCode, JSON.stringify(replay ? replay.response : response ?? null), replay?.statusCode ?? statusCode, JSON.stringify(replay ? replay.response : response ?? null), new Date().toISOString(), scopeKey)
}
