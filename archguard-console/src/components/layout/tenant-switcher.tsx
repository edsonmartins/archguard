// src/components/layout/tenant-switcher.tsx

import { Building } from 'lucide-react'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useTenantFilter, ALL_TENANTS } from '@/lib/hooks/use-tenant-filter'
import { useGroups } from '@/lib/hooks/use-groups'

export function TenantSwitcher() {
  const {
    activeTenant,
    setActiveTenant,
    canSwitchTenant,
    availableTenants,
    discoverTenants,
  } = useTenantFilter()
  const { data: groups } = useGroups()

  if (!canSwitchTenant) return null

  // Merge statically-known tenants with dynamically-discovered ones
  const discovered = groups ? discoverTenants(groups) : []
  const knownValues = new Set(availableTenants.map((t) => t.value))
  const merged = [
    ...availableTenants,
    ...discovered.filter((d) => !knownValues.has(d.value)),
  ]

  return (
    <div className="px-2 py-2">
      <div className="flex items-center gap-2 px-2 pb-1 text-xs font-medium text-muted-foreground">
        <Building className="h-3 w-3" />
        Tenant
      </div>
      <Select value={activeTenant} onValueChange={setActiveTenant}>
        <SelectTrigger className="h-8 text-xs">
          <SelectValue placeholder="Selecionar tenant" />
        </SelectTrigger>
        <SelectContent>
          {merged.map((t) => (
            <SelectItem key={t.value} value={t.value}>
              <span className="flex items-center gap-2">
                {t.label}
                {t.count !== undefined && (
                  <span className="text-muted-foreground">({t.count})</span>
                )}
              </span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
