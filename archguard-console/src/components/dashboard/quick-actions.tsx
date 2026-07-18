// src/components/dashboard/quick-actions.tsx
// Hub da admin unificada — preferir rotas do console a UIs stock externas

import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  UserPlus,
  UsersRound,
  KeyRound,
  Server,
  KeySquare,
  Gauge,
  Building2,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { PermissionGate } from '@/components/shared/permission-gate'

export function QuickActions() {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t('dashboard.quickActions.title')}</CardTitle>
        <p className="text-xs text-muted-foreground font-normal">
          {t('dashboard.quickActions.subtitle')}
        </p>
      </CardHeader>
      <CardContent className="grid gap-2 sm:grid-cols-2">
        <PermissionGate require="sites:read">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/platform">
              <Gauge className="h-4 w-4" />
              {t('dashboard.quickActions.platformHealth')}
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="gateways:read">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/gateways">
              <Server className="h-4 w-4" />
              {t('dashboard.quickActions.gateways')}
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="sites:read">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/sites">
              <Building2 className="h-4 w-4" />
              {t('dashboard.quickActions.sites')}
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="secrets:read">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/secrets">
              <KeySquare className="h-4 w-4" />
              {t('dashboard.quickActions.secrets')}
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="persons:create">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/identities/create">
              <UserPlus className="h-4 w-4" />
              {t('dashboard.quickActions.newPerson')}
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="groups:create">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/groups/create">
              <UsersRound className="h-4 w-4" />
              {t('dashboard.quickActions.newGroup')}
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="oauth2:create">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/oauth2/create">
              <KeyRound className="h-4 w-4" />
              {t('dashboard.quickActions.oauth2Client')}
            </Link>
          </Button>
        </PermissionGate>
      </CardContent>
    </Card>
  )
}
