// src/lib/hooks/use-tenant-filter.ts

import { useState, useCallback, useMemo } from 'react'
import { usePermissions } from './use-permissions'
import { extractTenantPrefix } from '@/lib/api/normalizers'
import type { Person, Group, OAuth2Client, ServiceAccount } from '@/lib/api/types/kanidm'

const STORAGE_KEY = 'archguard_active_tenant'

export type TenantOption = {
  value: string // tenant id (e.g. "tenant_rio_quality" or "acme") or "__all__"
  label: string
  count?: number
}

export const ALL_TENANTS = '__all__'

/**
 * Multi-tenant filtering for list pages (5.3 / 5.4).
 *
 * - SUPER_ADMIN: all tenants; optional switcher
 * - TENANT_ADMIN: only own tenant(s); forced scope
 * - Operator/Viewer with tenant_* membership: only own tenant(s) (read)
 * - No tenant claim + not system admin: empty lists (safe isolation)
 */
export function useTenantFilter() {
  const { isSystemAdmin, tenants } = usePermissions()

  const [activeTenant, setActiveTenantState] = useState<string>(() => {
    if (typeof window === 'undefined') return ALL_TENANTS
    try {
      return sessionStorage.getItem(STORAGE_KEY) ?? ALL_TENANTS
    } catch {
      return ALL_TENANTS
    }
  })

  const setActiveTenant = useCallback((tenant: string) => {
    setActiveTenantState(tenant)
    try {
      sessionStorage.setItem(STORAGE_KEY, tenant)
    } catch {
      // SSR or storage unavailable
    }
  }, [])

  const availableTenants = useMemo((): TenantOption[] => {
    if (isSystemAdmin) {
      return [
        { value: ALL_TENANTS, label: 'Todos os Tenants' },
        ...tenants.map((t) => ({ value: t, label: t })),
      ]
    }
    if (tenants.length > 0) {
      return tenants.map((t) => ({ value: t, label: t }))
    }
    return []
  }, [isSystemAdmin, tenants])

  const effectiveTenant = useMemo(() => {
    if (isSystemAdmin) return activeTenant
    if (tenants.length === 1) return tenants[0]!
    if (tenants.includes(activeTenant)) return activeTenant
    // multi-tenant non-admin: union mode
    return ALL_TENANTS
  }, [isSystemAdmin, tenants, activeTenant])

  // Non-system-admin is always scoped (even when "all my tenants")
  const isScoped = !isSystemAdmin
  const isFiltering =
    effectiveTenant !== ALL_TENANTS || (isScoped && tenants.length > 0)

  const personBelongsToTenant = useCallback(
    (person: Person, tenant: string): boolean => {
      const names = [...person.groups, ...person.groupNames]
      return names.some((g) => extractTenantPrefix(g) === tenant)
    },
    [],
  )

  const personBelongsToAny = useCallback(
    (person: Person, allowed: string[]): boolean => {
      if (allowed.length === 0) return false
      return allowed.some((t) => personBelongsToTenant(person, t))
    },
    [personBelongsToTenant],
  )

  const filterPersons = useCallback(
    (persons: Person[]): Person[] => {
      if (isSystemAdmin && effectiveTenant === ALL_TENANTS) return persons
      if (effectiveTenant !== ALL_TENANTS) {
        return persons.filter((p) => personBelongsToTenant(p, effectiveTenant))
      }
      // Scoped non-admin + "all my tenants"
      if (isScoped) {
        if (tenants.length === 0) return []
        return persons.filter((p) => personBelongsToAny(p, tenants))
      }
      return persons
    },
    [
      isSystemAdmin,
      isScoped,
      effectiveTenant,
      tenants,
      personBelongsToTenant,
      personBelongsToAny,
    ],
  )

  const groupBelongsToTenant = useCallback(
    (group: Group, tenant: string): boolean => {
      if (group.tenantName === tenant) return true
      return extractTenantPrefix(group.name) === tenant
    },
    [],
  )

  const filterGroups = useCallback(
    (groups: Group[]): Group[] => {
      if (isSystemAdmin && effectiveTenant === ALL_TENANTS) return groups
      if (effectiveTenant !== ALL_TENANTS) {
        return groups.filter((g) => groupBelongsToTenant(g, effectiveTenant))
      }
      if (isScoped) {
        if (tenants.length === 0) return []
        return groups.filter((g) =>
          tenants.some((t) => groupBelongsToTenant(g, t)),
        )
      }
      return groups
    },
    [isSystemAdmin, isScoped, effectiveTenant, tenants, groupBelongsToTenant],
  )

  const filterOAuth2 = useCallback(
    (clients: OAuth2Client[], allGroups?: Group[]): OAuth2Client[] => {
      if (isSystemAdmin && effectiveTenant === ALL_TENANTS) return clients
      const scopeTenants =
        effectiveTenant !== ALL_TENANTS ? [effectiveTenant] : tenants
      if (!isSystemAdmin && scopeTenants.length === 0) return []
      const tenantGroupIds = new Set(
        (allGroups ?? [])
          .filter((g) =>
            scopeTenants.some((t) => groupBelongsToTenant(g, t)),
          )
          .map((g) => g.id),
      )
      return clients.filter(
        (c) =>
          c.scopeMaps.some((sm) => tenantGroupIds.has(sm.groupId)) ||
          c.supplementalScopeMaps.some((sm) =>
            tenantGroupIds.has(sm.groupId),
          ),
      )
    },
    [isSystemAdmin, effectiveTenant, tenants, groupBelongsToTenant],
  )

  const filterServiceAccounts = useCallback(
    (accounts: ServiceAccount[]): ServiceAccount[] => {
      if (isSystemAdmin && effectiveTenant === ALL_TENANTS) return accounts
      const scopeTenants =
        effectiveTenant !== ALL_TENANTS ? [effectiveTenant] : tenants
      if (!isSystemAdmin && scopeTenants.length === 0) return []
      return accounts.filter((sa) => {
        const names = [...sa.groups, ...sa.groupNames]
        return names.some((g) => {
          const prefix = extractTenantPrefix(g)
          return prefix !== null && scopeTenants.includes(prefix)
        })
      })
    },
    [isSystemAdmin, effectiveTenant, tenants],
  )

  const discoverTenants = useCallback((groups: Group[]): TenantOption[] => {
    const tenantMap = new Map<string, number>()
    for (const g of groups) {
      const t = g.tenantName ?? extractTenantPrefix(g.name)
      if (t) {
        tenantMap.set(t, (tenantMap.get(t) ?? 0) + 1)
      }
    }
    return Array.from(tenantMap.entries())
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([name, count]) => ({ value: name, label: name, count }))
  }, [])

  return {
    activeTenant: effectiveTenant,
    setActiveTenant,
    isFiltering,
    availableTenants,
    canSwitchTenant: isSystemAdmin || tenants.length > 1,

    filterPersons,
    filterGroups,
    filterOAuth2,
    filterServiceAccounts,
    discoverTenants,
  }
}
