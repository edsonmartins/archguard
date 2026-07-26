// Generic key/value settings stored in SQLite (Manager-only ops; ADR-009 console).
// Do not store high-value secrets here — use OpenBao via BFF for those.

import { getDb } from './db'

export function getSetting(key: string): string {
  const row = getDb()
    .prepare(`SELECT value FROM manager_settings WHERE key = ?`)
    .get(key) as { value: string } | undefined
  return row?.value ?? ''
}

export function setSetting(
  key: string,
  value: string,
  actor = 'system',
): void {
  const now = new Date().toISOString()
  getDb()
    .prepare(
      `INSERT INTO manager_settings (key, value, updated_at, updated_by)
       VALUES (@key, @value, @now, @actor)
       ON CONFLICT(key) DO UPDATE SET
         value = excluded.value,
         updated_at = excluded.updated_at,
         updated_by = excluded.updated_by`,
    )
    .run({ key, value, now, actor })
}

export function getSettingsMap(keys: string[]): Record<string, string> {
  const out: Record<string, string> = {}
  for (const k of keys) out[k] = getSetting(k)
  return out
}
