// src/routes/_authed/dashboard.tsx

import { createFileRoute } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { StatsCards } from '@/components/dashboard/stats-cards'
import { SystemHealth } from '@/components/dashboard/system-health'
import { QuickActions } from '@/components/dashboard/quick-actions'
import { personApi } from '@/lib/api/kanidm-client'
import { groupApi } from '@/lib/api/kanidm-client'
import { oauth2Api } from '@/lib/api/kanidm-client'
import { systemApi } from '@/lib/api/kanidm-client'
import { vaultApi } from '@/lib/api/vault-client'
import { queryKeys } from '@/lib/utils/query-keys'

export const Route = createFileRoute('/_authed/dashboard')({
  component: DashboardPage,
})

function DashboardPage() {
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

  const vault = useQuery({
    queryKey: queryKeys.vault.status,
    queryFn: () => vaultApi.status(),
    staleTime: 30_000,
  })

  const isLoading =
    persons.isLoading ||
    groups.isLoading ||
    oauth2.isLoading

  const services = [
    {
      name: 'ArchGuard ID (Kanidm)',
      status: system.data
        ? ((system.data as Record<string, string>).state === 'ok'
            ? 'ok'
            : 'error')
        : system.isError
          ? 'unreachable'
          : ('ok' as const),
      version: (system.data as Record<string, string>)?.version,
    },
    {
      name: 'ArchGuard Vault',
      status: vault.data?.online
        ? 'ok'
        : vault.isError
          ? 'unreachable'
          : ('error' as const),
      version: vault.data?.version,
    },
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Dashboard</h1>
        <p className="text-muted-foreground">
          Visão geral do ArchGuard Console
        </p>
      </div>

      <StatsCards
        personsCount={persons.data?.length}
        groupsCount={groups.data?.length}
        oauth2Count={oauth2.data?.length}
        vaultOnline={vault.data?.online}
        isLoading={isLoading}
      />

      <div className="grid gap-6 lg:grid-cols-2">
        <SystemHealth
          services={services as { name: string; status: 'ok' | 'error' | 'unreachable'; version?: string }[]}
          isLoading={system.isLoading}
        />
        <QuickActions />
      </div>
    </div>
  )
}
