// src/components/shared/group-badge.tsx

import { Badge } from '@/components/ui/badge'
import { Building, Crown, Users, Lock } from 'lucide-react'
import { BUILTIN_GROUPS } from '@/lib/utils/constants'

interface GroupBadgeProps {
  name: string
  className?: string
}

function getGroupType(name: string) {
  if (BUILTIN_GROUPS.has(name)) return 'system'
  if (name.endsWith('_admins')) return 'admin'
  if (name.endsWith('_users')) return 'users'
  if (!name.includes('_')) return 'tenant'
  return 'custom'
}

const groupConfig: Record<
  string,
  {
    icon: React.ComponentType<{ className?: string }>
    variant: 'default' | 'secondary' | 'outline' | 'destructive'
  }
> = {
  system: { icon: Lock, variant: 'secondary' },
  admin: { icon: Crown, variant: 'default' },
  users: { icon: Users, variant: 'outline' },
  tenant: { icon: Building, variant: 'outline' },
  custom: { icon: Users, variant: 'outline' },
}

export function GroupBadge({ name, className }: GroupBadgeProps) {
  const type = getGroupType(name)
  const config = groupConfig[type] ?? groupConfig.custom!
  const Icon = config.icon

  return (
    <Badge variant={config.variant} className={className}>
      <Icon className="mr-1 h-3 w-3" />
      {name}
    </Badge>
  )
}
