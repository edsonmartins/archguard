// Módulo Segredos — OpenBao control plane (CP-4)

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { KeyRound, AlertTriangle, Unlock, Trash2, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Skeleton } from '@/components/ui/skeleton'
import {
  getOpenBaoOverviewFn,
  getOpenBaoStatusFn,
  revokeOpenBaoLeaseFn,
  unsealOpenBaoFn,
} from '@/server/openbao-fn'
import { usePermissions } from '@/lib/hooks/use-permissions'

export function SecretsPage() {
  const { t } = useTranslation()
  const { can } = usePermissions()
  const canManage = can('secrets:manage') || can('system:admin')
  const qc = useQueryClient()

  const status = useQuery({
    queryKey: ['openbao', 'status'],
    queryFn: () => getOpenBaoStatusFn(),
  })

  const sealed = status.data?.health?.sealed ?? status.data?.seal?.sealed
  const unsealed =
    status.data?.configured &&
    status.data?.token_configured &&
    sealed === false

  const overview = useQuery({
    queryKey: ['openbao', 'overview'],
    queryFn: () => getOpenBaoOverviewFn(),
    enabled: !!unsealed,
  })

  const unseal = useMutation({
    mutationFn: () => unsealOpenBaoFn(),
    onSuccess: (r) => {
      if (r.sealed) {
        toast.message(`Unseal progress: ${r.progress ?? '?'}`)
      } else {
        toast.success(t('secretsPage.unsealOk'))
      }
      void qc.invalidateQueries({ queryKey: ['openbao'] })
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const revoke = useMutation({
    mutationFn: (lease_id: string) =>
      revokeOpenBaoLeaseFn({ data: { lease_id } }),
    onSuccess: () => {
      toast.success(t('secretsPage.leaseRevoked'))
      void qc.invalidateQueries({ queryKey: ['openbao', 'overview'] })
    },
    onError: (e) => toast.error((e as Error).message),
  })

  return (
    <div className="space-y-6 p-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <KeyRound className="h-7 w-7 text-primary" />
            {t('secretsPage.title')}
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            {t('secretsPage.subtitle')}
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => void qc.invalidateQueries({ queryKey: ['openbao'] })}
        >
          <RefreshCw className="h-4 w-4 mr-1" /> Atualizar
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Status</CardTitle>
          <CardDescription>
            {status.data?.addr || t('secretsPage.addrMissing')}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {status.isLoading ? (
            <Skeleton className="h-8 w-64" />
          ) : status.isError ? (
            <p className="text-sm text-destructive">
              {(status.error as Error).message}
            </p>
          ) : !status.data?.configured ? (
            <p className="text-sm text-amber-700 dark:text-amber-400 flex gap-2">
              <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5" />
              {t('secretsPage.configureAddr')}
            </p>
          ) : (
            <>
              <div className="flex flex-wrap gap-2">
                <Badge
                  variant={
                    status.data.health?.initialized ? 'default' : 'secondary'
                  }
                >
                  {status.data.health?.initialized
                    ? 'initialized'
                    : 'not initialized'}
                </Badge>
                <Badge variant={sealed ? 'destructive' : 'default'}>
                  {sealed ? 'sealed' : 'unsealed'}
                </Badge>
                <Badge
                  variant={
                    status.data.token_configured ? 'outline' : 'destructive'
                  }
                >
                  {status.data.token_configured
                    ? 'token OK'
                    : 'token ausente'}
                </Badge>
                {status.data.health?.version && (
                  <Badge variant="secondary">
                    v{status.data.health.version}
                  </Badge>
                )}
              </div>
              {status.data.health?.cluster_name && (
                <p className="text-xs text-muted-foreground font-mono">
                  cluster {status.data.health.cluster_name}
                </p>
              )}
              {'error' in (status.data || {}) &&
                (status.data as { error?: string }).error && (
                  <p className="text-sm text-destructive">
                    {(status.data as { error?: string }).error}
                  </p>
                )}
              {sealed && canManage && (
                <Button
                  size="sm"
                  onClick={() => unseal.mutate()}
                  disabled={unseal.isPending}
                >
                  <Unlock className="h-4 w-4 mr-1" />
                  Unseal (chave do servidor)
                </Button>
              )}
            </>
          )}
        </CardContent>
      </Card>

      {unsealed && (
        <>
          <div className="grid gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>{t('secretsPage.engines')}</CardTitle>
              </CardHeader>
              <CardContent>
                {overview.isLoading ? (
                  <Skeleton className="h-20 w-full" />
                ) : overview.isError ? (
                  <p className="text-sm text-destructive">
                    {(overview.error as Error).message}
                  </p>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Path</TableHead>
                        <TableHead>Type</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {(overview.data?.mounts || []).map((m) => (
                        <TableRow key={m.path}>
                          <TableCell className="font-mono text-xs">
                            {m.path}
                          </TableCell>
                          <TableCell>
                            <Badge variant="outline">{m.type}</Badge>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>{t('secretsPage.authMethods')}</CardTitle>
              </CardHeader>
              <CardContent>
                {overview.isLoading ? (
                  <Skeleton className="h-20 w-full" />
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Path</TableHead>
                        <TableHead>Type</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {(overview.data?.auth || []).map((a) => (
                        <TableRow key={a.path}>
                          <TableCell className="font-mono text-xs">
                            {a.path}
                          </TableCell>
                          <TableCell>
                            <Badge variant="outline">{a.type}</Badge>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle>{t('secretsPage.policies')}</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-2">
              {(overview.data?.policies || []).map((p) => (
                <Badge key={p} variant="secondary">
                  {p}
                </Badge>
              ))}
            </CardContent>
          </Card>

          {overview.data?.jwt && (
            <Card>
              <CardHeader>
                <CardTitle>{t('secretsPage.jwtAuth')}</CardTitle>
                <CardDescription>
                  bound_issuer / default_role (pubkeys truncadas)
                </CardDescription>
              </CardHeader>
              <CardContent>
                <pre className="rounded bg-muted p-3 text-xs overflow-auto max-h-48">
                  {JSON.stringify(overview.data.jwt, null, 2)}
                </pre>
              </CardContent>
            </Card>
          )}

          <Card>
            <CardHeader>
              <CardTitle>
                Leases DB dinâmico (
                {overview.data?.db_leases?.role || 'lab-readonly'})
              </CardTitle>
              <CardDescription>
                {(overview.data?.db_leases?.lease_ids || []).length} lease(s)
                ativos listados
              </CardDescription>
            </CardHeader>
            <CardContent>
              {(overview.data?.db_leases?.lease_ids || []).length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  Nenhum lease listado (ou role vazia).
                </p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('secretsPage.leaseId')}</TableHead>
                      <TableHead className="w-24" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(overview.data?.db_leases?.lease_ids || []).map((id) => (
                      <TableRow key={id}>
                        <TableCell className="font-mono text-xs break-all">
                          {id}
                        </TableCell>
                        <TableCell>
                          {canManage && (
                            <Button
                              size="icon"
                              variant="ghost"
                              disabled={revoke.isPending}
                              onClick={() => revoke.mutate(id)}
                            >
                              <Trash2 className="h-4 w-4 text-destructive" />
                            </Button>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}
