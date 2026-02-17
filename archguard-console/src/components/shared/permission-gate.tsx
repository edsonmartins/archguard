// src/components/shared/permission-gate.tsx

import type { Permission } from '@/lib/auth/permissions'
import { usePermissions } from '@/lib/hooks/use-permissions'

interface PermissionGateProps {
  require: Permission | Permission[]
  any?: boolean
  fallback?: React.ReactNode
  children: React.ReactNode
}

export function PermissionGate({
  require,
  any,
  fallback,
  children,
}: PermissionGateProps) {
  const { can, canAny } = usePermissions()
  const perms = Array.isArray(require) ? require : [require]
  const allowed = any ? canAny(perms) : can(require)
  return allowed ? <>{children}</> : <>{fallback}</>
}
