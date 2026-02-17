// src/lib/hooks/use-audit.ts

import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import { kanidmApiFn } from '@/server/kanidm-proxy'
import { queryKeys } from '@/lib/utils/query-keys'
import type { AuditEvent } from '@/lib/api/types/kanidm'
import type { AuditFilters } from '@/lib/utils/validators'

async function fetchAuditEvents(filters: AuditFilters): Promise<AuditEvent[]> {
  try {
    const result = await kanidmApiFn({
      data: { method: 'GET', path: '/v1/recycle_bin' },
    })
    // Kanidm doesn't have a dedicated audit API in all versions;
    // this is a placeholder that returns mock data for now.
    // In production, this would integrate with Kanidm's audit log endpoint.
    return []
  } catch {
    return []
  }
}

export function useAuditEvents(
  filters: AuditFilters,
  options?: Partial<UseQueryOptions<AuditEvent[]>>,
) {
  return useQuery({
    queryKey: queryKeys.audit.list(filters),
    queryFn: () => fetchAuditEvents(filters),
    staleTime: 10_000,
    ...options,
  })
}
