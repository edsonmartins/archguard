// src/server/activity-log.ts
// Server-side activity log for tracking mutations via the console.
// Kanidm v1.9 has no public audit API, so we log mutation requests
// made through the proxy to provide basic activity visibility.

import { createServerFn } from '@tanstack/react-start'
import { getCookie } from '@tanstack/react-start/server'
import { decryptSession } from './session'
import type { SessionData } from './auth'
import type { ActivityLogEntry } from '@/lib/api/types/kanidm'

const MAX_ENTRIES = 500

// In-memory circular buffer (persists for server lifetime)
const entries: ActivityLogEntry[] = []

function getActor(): string {
  try {
    const sessionCookie = getCookie('archguard_session')
    if (!sessionCookie) return 'unknown'
    const session = decryptSession<SessionData>(sessionCookie)
    return session.user?.name ?? 'unknown'
  } catch {
    return 'unknown'
  }
}

/**
 * Record a mutation in the activity log.
 * Called by kanidm-proxy after successful write operations.
 */
export function recordActivity(
  method: string,
  path: string,
  actor: string,
  result: 'success' | 'error',
  errorMessage?: string,
) {
  // Derive human-readable action from method + path
  const action = deriveAction(method, path)
  const target = deriveTarget(path)

  const entry: ActivityLogEntry = {
    id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    timestamp: new Date().toISOString(),
    actor,
    action,
    method,
    path,
    target,
    result,
    errorMessage,
  }

  entries.unshift(entry)
  if (entries.length > MAX_ENTRIES) {
    entries.length = MAX_ENTRIES
  }
}

function deriveAction(method: string, path: string): string {
  const segments = path.split('/').filter(Boolean)
  const resource = segments[1] ?? '' // v1/person → person

  if (method === 'POST' && !path.includes('/_')) return `Criar ${resource}`
  if (method === 'DELETE') return `Excluir ${resource}`
  if (method === 'PUT' || method === 'PATCH') return `Atualizar ${resource}`
  if (path.includes('/_attr/member') && method === 'POST') return 'Adicionar membro'
  if (path.includes('/_attr/member') && method === 'DELETE') return 'Remover membro'
  if (path.includes('/_credential')) return 'Credencial'
  if (path.includes('/_scopemap')) return 'Scope Map'
  if (path.includes('/_claimmap')) return 'Claim Map'
  if (path.includes('/_api_token') && method === 'POST') return 'Gerar token'
  if (path.includes('/_api_token') && method === 'DELETE') return 'Revogar token'
  if (path.includes('/_revive')) return 'Restaurar da lixeira'
  return `${method} ${resource}`
}

function deriveTarget(path: string): string | undefined {
  // Extract the entity ID/name from the path
  // /v1/person/{id}/_attr/mail → {id}
  const segments = path.split('/').filter(Boolean)
  if (segments.length >= 3) {
    const id = segments[2]
    // Skip if it looks like a sub-resource prefix (starts with _)
    if (id && !id.startsWith('_')) return decodeURIComponent(id)
  }
  return undefined
}

// ── Server Functions ────────────────────────────

export const getActivityLogFn = createServerFn({ method: 'GET' }).handler(
  async (): Promise<ActivityLogEntry[]> => {
    return entries
  },
)

export { getActor }
