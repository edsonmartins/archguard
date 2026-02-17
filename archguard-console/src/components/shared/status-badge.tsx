// src/components/shared/status-badge.tsx

import { Badge } from '@/components/ui/badge'
import type { PersonStatus } from '@/lib/api/types/kanidm'

const statusConfig: Record<
  PersonStatus,
  { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' }
> = {
  active: { label: 'Ativo', variant: 'default' },
  expired: { label: 'Expirado', variant: 'destructive' },
  not_yet_valid: { label: 'Pendente', variant: 'outline' },
  locked: { label: 'Bloqueado', variant: 'destructive' },
  disabled: { label: 'Desabilitado', variant: 'secondary' },
}

interface StatusBadgeProps {
  status: PersonStatus
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const config = statusConfig[status] ?? statusConfig.active
  return <Badge variant={config.variant}>{config.label}</Badge>
}
