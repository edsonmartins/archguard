// Módulo Gateways — Warpgate + Guacamole (control plane CP-2 / CP-3)

import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Server } from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { usePermissions } from '@/lib/hooks/use-permissions'
import { WarpgatePanel } from './warpgate-panel'
import { GuacamolePanel } from './guacamole-panel'
import { SessionsPanel } from './sessions-panel'

export function GatewaysPage() {
  const { t } = useTranslation()
  const { can } = usePermissions()
  const canRead =
    can('gateways:read') ||
    can('gateways:manage') ||
    can('sites:update') ||
    can('system:admin')
  const canManage = can('gateways:manage') || can('system:admin')

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-2xl font-bold flex items-center gap-2">
          <Server className="h-7 w-7 text-primary" />
          {t('gatewaysPage.title')}
        </h1>
        <p className="text-sm text-muted-foreground mt-1">
          {t('gatewaysPage.subtitle')}{' '}
          <Link to="/sites" className="text-primary underline">
            {t('nav.sites')}
          </Link>
          .
        </p>
      </div>

      <Tabs defaultValue="warpgate">
        <TabsList>
          <TabsTrigger value="warpgate">{t('gatewaysPage.warpgate')}</TabsTrigger>
          <TabsTrigger value="guacamole">{t('gatewaysPage.guacamole')}</TabsTrigger>
          <TabsTrigger value="sessions">Sessões</TabsTrigger>
        </TabsList>

        <TabsContent value="warpgate" className="mt-4">
          <WarpgatePanel canRead={canRead} canManage={canManage} />
        </TabsContent>

        <TabsContent value="guacamole" className="mt-4">
          <GuacamolePanel canRead={canRead} canManage={canManage} />
        </TabsContent>

        <TabsContent value="sessions" className="mt-4">
          <SessionsPanel canRead={canRead} canManage={canManage} />
        </TabsContent>
      </Tabs>
    </div>
  )
}
