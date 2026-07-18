// Tenant scope for gateway inventory (Warpgate / Guacamole).
// Super-admin sees all; others only names/roles declared on their sites.

import type { SessionData } from './auth'
import {
  filterSitesByTenant,
  hasAnyPerm,
} from './session-guard'

export type GatewayTenantScope = {
  /** When true, no name filtering (platform admin). */
  unrestricted: boolean
  targetNames: Set<string>
  roleNames: Set<string>
  siteSlugs: string[]
}

/**
 * Build allow-list of target/role names for the session.
 * Empty sets (non-admin, no sites) → deny-by-default on gateway lists.
 */
export async function gatewayTenantScope(
  s: SessionData,
): Promise<GatewayTenantScope> {
  if (hasAnyPerm(s, ['system:admin'])) {
    return {
      unrestricted: true,
      targetNames: new Set(),
      roleNames: new Set(),
      siteSlugs: [],
    }
  }

  // dynamic import keeps pure filter unit tests free of DB/logger side-effects
  const { listSites } = await import('./sites')
  const sites = filterSitesByTenant(await listSites(), s)
  const targetNames = new Set<string>()
  const roleNames = new Set<string>()
  const siteSlugs: string[] = []

  for (const site of sites) {
    siteSlugs.push(site.slug)
    for (const t of site.targets || []) {
      if (t.nome) targetNames.add(t.nome)
    }
    for (const r of site.warpgate_roles || []) {
      if (r) roleNames.add(r)
    }
  }

  return { unrestricted: false, targetNames, roleNames, siteSlugs }
}

export function filterByTargetNames<T extends { name: string }>(
  items: T[],
  scope: GatewayTenantScope,
): T[] {
  if (scope.unrestricted) return items
  if (scope.targetNames.size === 0) return []
  return items.filter((i) => scope.targetNames.has(i.name))
}

export function filterByRoleNames<T extends { name: string }>(
  items: T[],
  scope: GatewayTenantScope,
): T[] {
  if (scope.unrestricted) return items
  if (scope.roleNames.size === 0) return []
  return items.filter((i) => scope.roleNames.has(i.name))
}
