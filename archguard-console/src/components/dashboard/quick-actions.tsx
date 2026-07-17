// src/components/dashboard/quick-actions.tsx
// Hub da admin unificada — preferir rotas do console a UIs stock externas

import { Link } from '@tanstack/react-router'
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
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Admin unificado</CardTitle>
        <p className="text-xs text-muted-foreground font-normal">
          Use o console para identidade, gateways e segredos. UIs stock (Warpgate /
          Guac / OpenBao) são break-glass.
        </p>
      </CardHeader>
      <CardContent className="grid gap-2 sm:grid-cols-2">
        <PermissionGate require="sites:read">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/platform">
              <Gauge className="h-4 w-4" />
              Saúde da plataforma
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="gateways:read">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/gateways">
              <Server className="h-4 w-4" />
              Gateways (WG / Guac)
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="sites:read">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/sites">
              <Building2 className="h-4 w-4" />
              Clientes / Sites
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="secrets:read">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/secrets">
              <KeySquare className="h-4 w-4" />
              Segredos (OpenBao)
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="persons:create">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/identities/create">
              <UserPlus className="h-4 w-4" />
              Nova pessoa
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="groups:create">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/groups/create">
              <UsersRound className="h-4 w-4" />
              Novo grupo
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="oauth2:create">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/oauth2/create">
              <KeyRound className="h-4 w-4" />
              Client OAuth2
            </Link>
          </Button>
        </PermissionGate>
      </CardContent>
    </Card>
  )
}
