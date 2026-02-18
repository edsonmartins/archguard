// src/components/oauth2/oauth2-list-page.tsx

import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import {
  KeyRound,
  Search,
  Globe,
  Shield,
  MoreHorizontal,
  Trash2,
  Eye,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { EmptyState } from '@/components/shared/empty-state'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { PermissionGate } from '@/components/shared/permission-gate'
import { useOAuth2Clients, useDeleteOAuth2Client } from '@/lib/hooks/use-oauth2'
import { useGroups } from '@/lib/hooks/use-groups'
import { useTenantFilter } from '@/lib/hooks/use-tenant-filter'
import type { OAuth2Client } from '@/lib/api/types/kanidm'

export function OAuth2ListPage() {
  const { data: clients, isLoading } = useOAuth2Clients()
  const { data: allGroups } = useGroups()
  const deleteClient = useDeleteOAuth2Client()
  const { filterOAuth2 } = useTenantFilter()
  const [search, setSearch] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<OAuth2Client | null>(null)

  const tenantFiltered = filterOAuth2(clients ?? [], allGroups)
  const filtered = tenantFiltered.filter(
    (c) =>
      c.name.toLowerCase().includes(search.toLowerCase()) ||
      c.displayName.toLowerCase().includes(search.toLowerCase()),
  )

  if (isLoading) {
    return <OAuth2ListSkeleton />
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">OAuth2 / SSO</h1>
          <p className="text-muted-foreground">
            Gerencie clientes OAuth2 e integrações SSO
          </p>
        </div>
        <PermissionGate require="oauth2:create">
          <Button asChild>
            <Link to="/oauth2/create" as={undefined}>
              <KeyRound className="mr-2 h-4 w-4" />
              Novo Client
            </Link>
          </Button>
        </PermissionGate>
      </div>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Buscar clientes..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="pl-10"
        />
      </div>

      {!filtered || filtered.length === 0 ? (
        <EmptyState
          icon={KeyRound}
          title="Nenhum cliente OAuth2"
          description="Crie um cliente OAuth2 para integrar aplicações via SSO."
          action={{ label: 'Novo Client', to: '/oauth2/create' }}
        />
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {filtered.map((client) => (
            <Card key={client.id} className="relative">
              <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-2">
                <div className="flex items-center gap-2">
                  {client.type === 'public' ? (
                    <Globe className="h-5 w-5 text-blue-500" />
                  ) : (
                    <Shield className="h-5 w-5 text-green-500" />
                  )}
                  <CardTitle className="text-base">{client.displayName}</CardTitle>
                </div>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon" className="h-8 w-8">
                      <MoreHorizontal className="h-4 w-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem asChild>
                      <Link
                        to="/oauth2/$clientId"
                        params={{ clientId: client.id }}
                      >
                        <Eye className="mr-2 h-4 w-4" />
                        Ver Detalhes
                      </Link>
                    </DropdownMenuItem>
                    <PermissionGate require="oauth2:delete">
                      <DropdownMenuItem
                        className="text-destructive"
                        onClick={() => setDeleteTarget(client)}
                      >
                        <Trash2 className="mr-2 h-4 w-4" />
                        Excluir
                      </DropdownMenuItem>
                    </PermissionGate>
                  </DropdownMenuContent>
                </DropdownMenu>
              </CardHeader>
              <CardContent>
                <div className="space-y-2 text-sm">
                  <div className="flex items-center gap-2">
                    <Badge
                      variant={client.type === 'public' ? 'outline' : 'secondary'}
                    >
                      {client.type === 'public' ? 'Public (PKCE)' : 'Basic (Confidential)'}
                    </Badge>
                  </div>
                  <p className="text-xs font-mono text-muted-foreground">
                    {client.name}
                  </p>
                  {client.landingUrl && (
                    <p className="truncate text-xs text-muted-foreground">
                      {client.landingUrl}
                    </p>
                  )}
                  <div className="flex items-center gap-1 pt-1">
                    {client.scopeMaps.length > 0 && (
                      <Badge variant="outline" className="text-xs">
                        {client.scopeMaps.length} scope maps
                      </Badge>
                    )}
                    {client.redirectUrls.length > 0 && (
                      <Badge variant="outline" className="text-xs">
                        {client.redirectUrls.length} redirects
                      </Badge>
                    )}
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Excluir Cliente OAuth2"
        description={`Esta ação é irreversível. O cliente "${deleteTarget?.displayName}" será removido e todas as integrações deixarão de funcionar.`}
        confirmText={deleteTarget?.name ?? ''}
        destructive
        isLoading={deleteClient.isPending}
        onConfirm={() => {
          if (deleteTarget) {
            deleteClient.mutate(deleteTarget.id, {
              onSuccess: () => setDeleteTarget(null),
            })
          }
        }}
      />
    </div>
  )
}

function OAuth2ListSkeleton() {
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <Skeleton className="h-9 w-48" />
          <Skeleton className="mt-2 h-5 w-72" />
        </div>
        <Skeleton className="h-10 w-32" />
      </div>
      <Skeleton className="h-10 w-80" />
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-40" />
        ))}
      </div>
    </div>
  )
}
