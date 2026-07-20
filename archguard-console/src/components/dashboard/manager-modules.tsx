// ArchGate Manager — module hub (ADR-009A)
// Config surfaces only; sessions live in Connect / UnifiedUI.

import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  Building2,
  Server,
  KeySquare,
  Users,
  Gauge,
  FileText,
  Cloud,
  MonitorSmartphone,
} from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { PermissionGate } from '@/components/shared/permission-gate'
import type { Permission } from '@/lib/auth/permissions'

type Module = {
  key: string
  to: string
  icon: React.ComponentType<{ className?: string }>
  permission: Permission | null
  external?: boolean
}

const modules: Module[] = [
  {
    key: 'sites',
    to: '/sites',
    icon: Building2,
    permission: 'sites:read',
  },
  {
    key: 'gateways',
    to: '/gateways',
    icon: Server,
    permission: 'gateways:read',
  },
  {
    key: 'secrets',
    to: '/secrets',
    icon: KeySquare,
    permission: 'secrets:read',
  },
  {
    key: 'identity',
    to: '/identities',
    icon: Users,
    permission: 'persons:read',
  },
  {
    key: 'platform',
    to: '/platform',
    icon: Gauge,
    permission: 'sites:read',
  },
  {
    key: 'audit',
    to: '/audit',
    icon: FileText,
    permission: 'audit:read',
  },
  {
    key: 'axis',
    to: '/integrations/mentors-axis',
    icon: Cloud,
    permission: 'sites:update',
  },
]

export function ManagerModules() {
  const { t } = useTranslation()

  return (
    <div className="space-y-3">
      <div>
        <h2 className="text-lg font-semibold tracking-tight">
          {t('dashboard.modules.title')}
        </h2>
        <p className="text-sm text-muted-foreground">{t('dashboard.modules.subtitle')}</p>
      </div>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {modules.map((m) => {
          const Icon = m.icon
          const card = (
            <Link to={m.to} className="block h-full">
              <Card className="h-full transition-colors hover:border-primary/40 hover:bg-muted/30">
                <CardHeader className="pb-2">
                  <div className="flex items-center gap-2">
                    <Icon className="h-4 w-4 text-primary" />
                    <CardTitle className="text-sm font-semibold">
                      {t(`dashboard.modules.${m.key}.title`)}
                    </CardTitle>
                  </div>
                  <CardDescription className="text-xs leading-snug">
                    {t(`dashboard.modules.${m.key}.desc`)}
                  </CardDescription>
                </CardHeader>
              </Card>
            </Link>
          )
          if (!m.permission) return <div key={m.key}>{card}</div>
          return (
            <PermissionGate key={m.key} require={m.permission}>
              {card}
            </PermissionGate>
          )
        })}
        <Card className="border-dashed">
          <CardHeader className="pb-2">
            <div className="flex items-center gap-2">
              <MonitorSmartphone className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-sm font-semibold text-muted-foreground">
                {t('dashboard.modules.connect.title')}
              </CardTitle>
            </div>
            <CardDescription className="text-xs leading-snug">
              {t('dashboard.modules.connect.desc')}
            </CardDescription>
          </CardHeader>
          <CardContent className="pt-0">
            <p className="text-[11px] text-muted-foreground font-mono">
              clients/archgate-connect
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
