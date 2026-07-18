// Server-side Kanidm admin helpers (service account token — never browser).
// Used by control plane to ensure tenant groups exist (ADR-004).

import { logger } from './logger'

const KANIDM_URL = (
  process.env.ARCHGUARD_ID_URL || 'https://localhost:8443'
).replace(/\/$/, '')
const KANIDM_SA_TOKEN = process.env.ARCHGUARD_SA_TOKEN || ''

function configured(): boolean {
  return Boolean(KANIDM_URL && KANIDM_SA_TOKEN)
}

async function kanidmFetch(
  method: string,
  path: string,
  body?: unknown,
): Promise<{ status: number; text: string }> {
  if (!configured()) {
    throw new Error('Kanidm SA não configurado (ARCHGUARD_ID_URL / ARCHGUARD_SA_TOKEN)')
  }
  const res = await fetch(`${KANIDM_URL}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${KANIDM_SA_TOKEN}`,
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  const text = await res.text()
  return { status: res.status, text }
}

/**
 * Ensure a group exists in Kanidm (idempotent).
 * Returns created | exists | error.
 */
export async function ensureKanidmGroup(
  name: string,
  description?: string,
): Promise<{ name: string; action: 'created' | 'exists' | 'skipped' | 'error'; error?: string }> {
  const group = name.trim()
  if (!group) {
    return { name, action: 'error', error: 'empty group name' }
  }
  if (!configured()) {
    logger.warn('ensureKanidmGroup: SA not configured — skip')
    return { name: group, action: 'skipped', error: 'kanidm SA not configured' }
  }

  try {
    const get = await kanidmFetch('GET', `/v1/group/${encodeURIComponent(group)}`)
    if (get.status >= 200 && get.status < 300) {
      return { name: group, action: 'exists' }
    }
    // 404 → create
    if (get.status !== 404) {
      return {
        name: group,
        action: 'error',
        error: `GET group HTTP ${get.status}: ${get.text.slice(0, 200)}`,
      }
    }

    const create = await kanidmFetch('POST', '/v1/group', {
      attrs: {
        name: [group],
        ...(description
          ? { description: [description] }
          : { description: [`ArchGate tenant group ${group}`] }),
      },
    })
    if (create.status >= 200 && create.status < 300) {
      logger.info({ group }, 'kanidm group created')
      return { name: group, action: 'created' }
    }
    // race: created elsewhere
    if (create.status === 409 || create.text.toLowerCase().includes('already')) {
      return { name: group, action: 'exists' }
    }
    return {
      name: group,
      action: 'error',
      error: `POST group HTTP ${create.status}: ${create.text.slice(0, 200)}`,
    }
  } catch (e) {
    return { name: group, action: 'error', error: (e as Error).message }
  }
}

/** Normalize tenant_group → ensure Kanidm group exists. */
export async function ensureTenantGroup(
  tenantGroup: string,
  cliente?: string,
): Promise<{ name: string; action: 'created' | 'exists' | 'skipped' | 'error'; error?: string }> {
  const name = tenantGroup.includes('@')
    ? tenantGroup.split('@')[0]!
    : tenantGroup
  const desc = cliente
    ? `ArchGate tenant for ${cliente} (${name})`
    : `ArchGate tenant ${name}`
  return ensureKanidmGroup(name, desc)
}

export function kanidmAdminConfigured(): boolean {
  return configured()
}
