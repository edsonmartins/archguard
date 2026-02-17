// src/lib/utils/query-keys.ts

import type { SearchParams, AuditFilters } from './validators'

export const queryKeys = {
  persons: {
    all: ['persons'] as const,
    list: (params?: SearchParams) => ['persons', 'list', params] as const,
    detail: (id: string) => ['persons', 'detail', id] as const,
    credentials: (id: string) => ['persons', 'credentials', id] as const,
  },
  groups: {
    all: ['groups'] as const,
    list: (params?: SearchParams) => ['groups', 'list', params] as const,
    detail: (id: string) => ['groups', 'detail', id] as const,
    members: (id: string) => ['groups', 'members', id] as const,
  },
  oauth2: {
    all: ['oauth2'] as const,
    list: () => ['oauth2', 'list'] as const,
    detail: (id: string) => ['oauth2', 'detail', id] as const,
    secret: (id: string) => ['oauth2', 'secret', id] as const,
  },
  serviceAccounts: {
    all: ['service-accounts'] as const,
    list: () => ['service-accounts', 'list'] as const,
    detail: (id: string) => ['service-accounts', 'detail', id] as const,
  },
  system: {
    status: ['system', 'status'] as const,
    domain: ['system', 'domain'] as const,
  },
  audit: {
    list: (filters: AuditFilters) => ['audit', 'list', filters] as const,
  },
  vault: {
    status: ['vault', 'status'] as const,
  },
  dashboard: {
    stats: ['dashboard', 'stats'] as const,
    activity: ['dashboard', 'activity'] as const,
    health: ['dashboard', 'health'] as const,
  },
}
