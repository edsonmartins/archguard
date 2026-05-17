// src/lib/hooks/use-activity-log.ts

import { useQuery } from '@tanstack/react-query'
import { getActivityLogFn } from '@/server/activity-log-fn'
import type { ActivityLogEntry } from '@/lib/api/types/kanidm'

export function useActivityLog() {
  return useQuery<ActivityLogEntry[]>({
    queryKey: ['activityLog'],
    queryFn: () => getActivityLogFn(),
    staleTime: 10_000,
    refetchInterval: 30_000,
  })
}
