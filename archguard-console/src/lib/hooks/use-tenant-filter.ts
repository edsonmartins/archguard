// src/lib/hooks/use-tenant-filter.ts

import { useState, useCallback, useMemo } from 'react'
import { usePermissions } from './use-permissions'
import { extractTenantPrefix } from '@/lib/api/normalizers'
import type { Person, Group, OAuth2Client, ServiceAccount } from '@/lib/api/types/kanidm'

const STORAGE_KEY = 'archguard_active_tenant'

export type TenantOption = {
  value: string       // tenant prefix (e.g. "acme") or "__all__"
  label: string       // display name
  count?: number      // optional entity count
}

export const ALL_TENANTS = '__all__'

/**
 * Hook that provides multi-tenant filtering for list pages.
 *
 * - SUPER_ADMIN: sees all tenants, can switch between them
 * - TENANT_ADMIN: sees only their own tenant(s)
 * - Others: no filtering applied (see all that permissions allow)
 */
export function useTenantFilter() {
  const { isSystemAdmin, isTenantAdmin, tenants } = usePermissions()

  // Restore last selection from sessionStorage
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

  // Available tenants for the switcher
  const availableTenants = useMemo((): TenantOption[] => {
    if (isSystemAdmin) {
      // Super Admin sees "All" + any known tenants
      return [
        { value: ALL_TENANTS, label: 'Todos os Tenants' },
        ...tenants.map((t) => ({ value: t, label: t })),
      ]
    }
    if (isTenantAdmin && tenants.length > 0) {
      // Tenant Admin: only their tenants (no "All" option)
      return tenants.map((t) => ({ value: t, label: t }))
    }
    return []
  }, [isSystemAdmin, isTenantAdmin, tenants])

  // Effective tenant: for Tenant Admin with single tenant, always filter by it
  const effectiveTenant = useMemo(() => {
    if (isSystemAdmin) return activeTenant
    if (isTenantAdmin && tenants.length === 1) return tenants[0]!
    if (isTenantAdmin && tenants.includes(activeTenant)) return activeTenant
    return ALL_TENANTS
  }, [isSystemAdmin, isTenantAdmin, tenants, activeTenant])

  const isFiltering = effectiveTenant !== ALL_TENANTS

  // ── Filter Functions ──────────────────────────────

  /**
   * Check if a person belongs to a tenant.
   * A person belongs to tenant "acme" if any of their groups starts with "acme_" or equals "acme".
   */
  const personBelongsToTenant = useCallback(
    (person: Person, tenant: string): boolean => {
      return person.groups.some((g) => {
        const prefix = extractTenantPrefix(g)
        return prefix === tenant
      })
    },
    [],
  )

  const filterPersons = useCallback(
    (persons: Person[]): Person[] => {
      if (!isFiltering) return persons
      return persons.filter((p) => personBelongsToTenant(p, effectiveTenant))
    },
    [isFiltering, effectiveTenant, personBelongsToTenant],
  )

  /**
   * Check if a group belongs to a tenant.
   * Uses the normalized tenantName from the Group object.
   */
  const filterGroups = useCallback(
    (groups: Group[]): Group[] => {
      if (!isFiltering) return groups
      return groups.filter((g) => g.tenantName === effectiveTenant)
    },
    [isFiltering, effectiveTenant],
  )

  /**
   * Filter OAuth2 clients by tenant.
   * A client belongs to a tenant if any of its scope map groups belong to that tenant.
   */
  const filterOAuth2 = useCallback(
    (clients: OAuth2Client[], allGroups?: Group[]): OAuth2Client[] => {
      if (!isFiltering) return clients
      // Build a set of group IDs that belong to the active tenant
      const tenantGroupIds = new Set(
        (allGroups ?? [])
          .filter((g) => g.tenantName === effectiveTenant)
          .map((g) => g.id),
      )
      return clients.filter((c) =>
        c.scopeMaps.some((sm) => tenantGroupIds.has(sm.groupId)) ||
        c.supplementalScopeMaps.some((sm) => tenantGroupIds.has(sm.groupId)),
      )
    },
    [isFiltering, effectiveTenant],
  )

  /**
   * Filter service accounts by tenant.
   * A SA belongs to a tenant if any of its groups belong to that tenant.
   */
  const filterServiceAccounts = useCallback(
    (accounts: ServiceAccount[]): ServiceAccount[] => {
      if (!isFiltering) return accounts
      return accounts.filter((sa) =>
        sa.groups.some((g) => {
          const prefix = extractTenantPrefix(g)
          return prefix === effectiveTenant
        }),
      )
    },
    [isFiltering, effectiveTenant],
  )

  /**
   * Discover all tenant prefixes from a list of groups.
   * Useful for populating the TenantSwitcher dynamically.
   */
  const discoverTenants = useCallback(
    (groups: Group[]): TenantOption[] => {
      const tenantMap = new Map<string, number>()
      for (const g of groups) {
        if (g.tenantName) {
          tenantMap.set(g.tenantName, (tenantMap.get(g.tenantName) ?? 0) + 1)
        }
      }
      return Array.from(tenantMap.entries())
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([name, count]) => ({ value: name, label: name, count }))
    },
    [],
  )

  return {
    // State
    activeTenant: effectiveTenant,
    setActiveTenant,
    isFiltering,
    availableTenants,
    canSwitchTenant: isSystemAdmin || (isTenantAdmin && tenants.length > 1),

    // Filter functions
    filterPersons,
    filterGroups,
    filterOAuth2,
    filterServiceAccounts,
    discoverTenants,
  }
}
