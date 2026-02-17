// src/lib/api/types/shared.ts

export interface ApiError {
  status: number
  message: string
  code?: string
  details?: Record<string, unknown>
}

export interface BulkAction<T> {
  label: string
  icon?: React.ComponentType<{ className?: string }>
  action: (items: T[]) => Promise<void>
  requireConfirm?: boolean
}

export interface PaginatedResult<T> {
  data: T[]
  total: number
  page: number
  pageSize: number
}
