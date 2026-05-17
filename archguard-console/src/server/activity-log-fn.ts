// src/server/activity-log-fn.ts
//
// Client-facing server function for the activity log. Lives in its own file
// so the TanStack Start plugin can rewrite it to an RPC stub on the client
// without dragging the better-sqlite3 binding (used by the helpers in
// activity-log.ts) into the client bundle.

import { createServerFn } from '@tanstack/react-start'
import { queryActivityLog } from './activity-log'
import type { ActivityLogEntry } from '@/lib/api/types/kanidm'

export const getActivityLogFn = createServerFn({ method: 'GET' }).handler(
  async (): Promise<ActivityLogEntry[]> => queryActivityLog({ limit: 500 }),
)
