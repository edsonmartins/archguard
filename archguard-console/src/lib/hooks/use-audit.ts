// src/lib/hooks/use-audit.ts

import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import { kanidmApiFn } from '@/server/kanidm-proxy'
import { queryKeys } from '@/lib/utils/query-keys'
import type { AuditEvent } from '@/lib/api/types/kanidm'
import type { AuditFilters } from '@/lib/utils/validators'

async function fetchAuditEvents(_filters: AuditFilters): Promise<AuditEvent[]> {
  try {
    await kanidmApiFn({
      data: { method: 'GET', path: '/v1/recycle_bin' },
    })
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
