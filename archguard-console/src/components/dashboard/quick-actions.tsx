// src/components/dashboard/quick-actions.tsx

import { Link } from '@tanstack/react-router'
import { UserPlus, UsersRound, KeyRound, KeySquare } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { PermissionGate } from '@/components/shared/permission-gate'

export function QuickActions() {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Ações Rápidas</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-2 sm:grid-cols-2">
        <PermissionGate require="persons:create">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/identities/create">
              <UserPlus className="h-4 w-4" />
              Nova Pessoa
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="groups:create">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/groups/create">
              <UsersRound className="h-4 w-4" />
              Novo Grupo
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="oauth2:create">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/oauth2/create">
              <KeyRound className="h-4 w-4" />
              Novo Client OAuth2
            </Link>
          </Button>
        </PermissionGate>
        <PermissionGate require="persons:credentials">
          <Button asChild variant="outline" className="justify-start gap-2">
            <Link to="/identities">
              <KeySquare className="h-4 w-4" />
              Reset Credencial
            </Link>
          </Button>
        </PermissionGate>
      </CardContent>
    </Card>
  )
}
