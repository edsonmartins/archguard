// src/routes/_authed/dashboard.tsx

import { createFileRoute } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { StatsCards } from '@/components/dashboard/stats-cards'
import { SystemHealth } from '@/components/dashboard/system-health'
import { QuickActions } from '@/components/dashboard/quick-actions'
import { ManagerModules } from '@/components/dashboard/manager-modules'
import { personApi, groupApi, oauth2Api, systemApi } from '@/lib/api/kanidm-client'
import { getOpenBaoStatusFn } from '@/server/openbao-fn'
import { queryKeys } from '@/lib/utils/query-keys'
import { useTenantFilter } from '@/lib/hooks/use-tenant-filter'

export const Route = createFileRoute('/_authed/dashboard')({
  component: DashboardPage,
})

/** Kanidm /status returns bare `true`, not `{ state: "ok" }`. */
function isKanidmOnline(data: unknown): boolean {
  if (data === true || data === 'true') return true
  if (data && typeof data === 'object') {
    const d = data as Record<string, unknown>
    if (d.state === 'ok' || d.ok === true) return true
  }
  return false
}

function DashboardPage() {
  const { t } = useTranslation()
  const { filterPersons, filterGroups, filterOAuth2 } = useTenantFilter()

  const persons = useQuery({
    queryKey: queryKeys.persons.all,
    queryFn: () => personApi.list(),
    staleTime: 30_000,
  })

  const groups = useQuery({
    queryKey: queryKeys.groups.all,
    queryFn: () => groupApi.list(),
    staleTime: 60_000,
  })

  const oauth2 = useQuery({
    queryKey: queryKeys.oauth2.list(),
    queryFn: () => oauth2Api.list(),
    staleTime: 5 * 60_000,
  })

  const system = useQuery({
    queryKey: queryKeys.system.status,
    queryFn: () => systemApi.status(),
    staleTime: 10_000,
  })

  // OpenBao (ArchGate secrets) — not AliasVault
  const openbao = useQuery({
    queryKey: ['openbao', 'status', 'dashboard'],
    queryFn: () => getOpenBaoStatusFn(),
    staleTime: 30_000,
  })

  const filteredPersons = filterPersons(persons.data ?? [])
  const filteredGroups = filterGroups(groups.data ?? [])
  const filteredOAuth2 = filterOAuth2(oauth2.data ?? [], groups.data)

  const isLoading =
    persons.isLoading ||
    groups.isLoading ||
    oauth2.isLoading

  const kanidmOk = !system.isError && isKanidmOnline(system.data)
  const baoHealth = openbao.data?.health as
    | { sealed?: boolean; initialized?: boolean; version?: string }
    | null
    | undefined
  const openbaoOk =
    !openbao.isError &&
    Boolean(openbao.data?.configured) &&
    baoHealth != null &&
    baoHealth.sealed === false

  const services = [
    {
      name: t('dashboard.health.kanidm'),
      status: system.isLoading
        ? ('ok' as const)
        : system.isError
          ? ('unreachable' as const)
          : kanidmOk
            ? ('ok' as const)
            : ('error' as const),
      version:
        system.data && typeof system.data === 'object'
          ? String((system.data as Record<string, string>).version || '')
          : undefined,
    },
    {
      name: t('dashboard.health.openbao'),
      status: openbao.isLoading
        ? ('ok' as const)
        : openbao.isError
          ? ('unreachable' as const)
          : openbaoOk
            ? ('ok' as const)
            : openbao.data?.configured
              ? ('error' as const)
              : ('unreachable' as const),
      version: baoHealth?.version,
    },
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">{t('dashboard.title')}</h1>
        <p className="text-muted-foreground">{t('dashboard.subtitle')}</p>
      </div>

      <StatsCards
        personsCount={filteredPersons.length}
        groupsCount={filteredGroups.length}
        oauth2Count={filteredOAuth2.length}
        vaultOnline={openbaoOk}
        isLoading={isLoading}
      />

      <ManagerModules />

      <div className="grid gap-6 lg:grid-cols-2">
        <SystemHealth
          services={
            services as {
              name: string
              status: 'ok' | 'error' | 'unreachable'
              version?: string
            }[]
          }
          isLoading={system.isLoading || openbao.isLoading}
        />
        <QuickActions />
      </div>
    </div>
  )
}
