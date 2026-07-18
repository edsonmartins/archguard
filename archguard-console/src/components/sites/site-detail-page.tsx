import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeft,
  Pencil,
  Building2,
  RefreshCw,
  Download,
  KeyRound,
} from 'lucide-react'
import { toast } from 'sonner'
import { enumLabel } from '@/lib/i18n/labels'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { storeTargetSecretFn } from '@/server/target-secret-fn'
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
import { PermissionGate } from '@/components/shared/permission-gate'
import { useExportSiteYaml, useSite, siteKeys } from '@/lib/hooks/use-sites'
import { applySiteTargetsFn } from '@/server/warpgate-fn'
import { usePermissions } from '@/lib/hooks/use-permissions'
import { ConnectorStatusPanel } from '@/components/sites/connector-status-panel'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

export function SiteDetailPage({ slug }: { slug: string }) {
  const { t } = useTranslation()
  const { data: site, isLoading, error } = useSite(slug)
  const { can } = usePermissions()
  const qc = useQueryClient()
  const exportOne = useExportSiteYaml()
  const [applyLog, setApplyLog] = useState<string>('')
  const [secretTarget, setSecretTarget] = useState<string | null>(null)
  const [secretPass, setSecretPass] = useState('')
  const [secretUser, setSecretUser] = useState('')
  const apply = useMutation({
    mutationFn: () => applySiteTargetsFn({ data: { slug } }),
    onSuccess: (res) => {
      const lines = res.results.map(
        (r) =>
          `${r.ok ? 'OK' : 'FAIL'} [${r.engine}] ${r.name}${r.error ? ': ' + r.error : ''}`,
      )
      setApplyLog(lines.join('\n'))
      if (res.allOk) {
        toast.success('Targets sincronizados (Warpgate / Guacamole)')
        void qc.invalidateQueries({ queryKey: siteKeys.detail(slug) })
        void qc.invalidateQueries({ queryKey: ['warpgate'] })
        void qc.invalidateQueries({ queryKey: ['guacamole'] })
      } else {
        toast.error('Alguns targets falharam — veja o log')
      }
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const storeSecret = useMutation({
    mutationFn: () =>
      storeTargetSecretFn({
        data: {
          slug,
          target_name: secretTarget!,
          password: secretPass,
          username: secretUser || undefined,
          apply: true,
        },
      }),
    onSuccess: (res) => {
      toast.success(res.message)
      setSecretTarget(null)
      setSecretPass('')
      setSecretUser('')
      void qc.invalidateQueries({ queryKey: siteKeys.detail(slug) })
      if (res.apply?.results) {
        setApplyLog(
          res.apply.results
            .map(
              (r) =>
                `${r.ok ? 'OK' : 'FAIL'} ${r.name}${r.error ? ': ' + r.error : ''}`,
            )
            .join('\n'),
        )
      }
    },
    onError: (e) => toast.error((e as Error).message),
  })

  if (isLoading) {
    return (
      <div className="p-6 space-y-3">
        <Skeleton className="h-8 w-72" />
        <Skeleton className="h-48 w-full" />
      </div>
    )
  }

  if (error || !site) {
    return (
      <div className="p-6">
        {(error as Error)?.message || t('sites.notFound')}{' '}
        <Link to="/sites" className="text-primary underline">
          Voltar
        </Link>
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6 max-w-4xl">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex items-start gap-3">
          <Button variant="ghost" size="icon" asChild>
            <Link to="/sites">
              <ArrowLeft className="h-4 w-4" />
            </Link>
          </Button>
          <div>
            <h1 className="text-2xl font-bold flex items-center gap-2">
              <Building2 className="h-6 w-6 text-primary" />
              {site.cliente}
            </h1>
            <p className="text-sm text-muted-foreground font-mono">
              {site.slug} · {site.tenant_group}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={exportOne.isPending}
            onClick={() =>
              exportOne.mutate(site.slug, {
                onSuccess: () => toast.success('YAML exportado'),
                onError: (e) => toast.error((e as Error).message),
              })
            }
          >
            <Download className="h-4 w-4 mr-1" />
            YAML
          </Button>
          {(can('gateways:manage') || can('sites:update')) &&
            site.targets.length > 0 && (
              <Button
                variant="secondary"
                disabled={apply.isPending}
                onClick={() => apply.mutate()}
              >
                <RefreshCw
                  className={`h-4 w-4 mr-2 ${apply.isPending ? 'animate-spin' : ''}`}
                />
                {t('sites.syncGateways')}
              </Button>
            )}
          <PermissionGate require="sites:update">
            <Button asChild>
              <Link to="/sites/$slug/edit" params={{ slug: site.slug }}>
                <Pencil className="h-4 w-4 mr-2" />
                {t('common.edit')}
              </Link>
            </Button>
          </PermissionGate>
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        <Badge variant="outline">
          {enumLabel(t, 'ambiente', site.ambiente)}
        </Badge>
        <Badge>{enumLabel(t, 'tipo', site.tipo)}</Badge>
        <Badge variant={site.stack === 'a_confirmar' ? 'destructive' : 'default'}>
          {enumLabel(t, 'stack', site.stack)}
        </Badge>
        {site.inventariado && (
          <Badge variant="secondary">{t('sites.inventoried')}</Badge>
        )}
        {site.connector_deployed && (
          <Badge variant="secondary">connector</Badge>
        )}
        {site.smoke_operador && <Badge variant="secondary">{t('sites.smokeOk')}</Badge>}
      </div>

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">{t('sites.overview')}</TabsTrigger>
          <TabsTrigger value="connector">{t('sites.connector')}</TabsTrigger>
          <TabsTrigger value="targets">{t('sites.targets')}</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="mt-4 space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Conectividade</CardTitle>
              <CardDescription>
                Connector e stack VPN (sem segredos).
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-2 text-sm sm:grid-cols-2">
              <div>
                <span className="text-muted-foreground">Connector ID</span>
                <div className="font-mono">{site.connector_id || '—'}</div>
              </div>
              <div>
                <span className="text-muted-foreground">Subnets</span>
                <div className="font-mono">
                  {site.subnets.length ? site.subnets.join(', ') : '—'}
                </div>
              </div>
              <div className="sm:col-span-2">
                <span className="text-muted-foreground">Metadados stack</span>
                <pre className="mt-1 rounded bg-muted p-2 text-xs overflow-auto">
                  {JSON.stringify(site.stack_meta || {}, null, 2)}
                </pre>
              </div>
              {site.notas && (
                <div className="sm:col-span-2">
                  <span className="text-muted-foreground">Notas</span>
                  <p className="mt-1 whitespace-pre-wrap">{site.notas}</p>
                </div>
              )}
              <div className="sm:col-span-2 text-xs text-muted-foreground">
                Atualizado {site.updated_at}
                {site.updated_by ? ` por ${site.updated_by}` : ''}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="connector" className="mt-4">
          <ConnectorStatusPanel slug={site.slug} />
        </TabsContent>

        <TabsContent value="targets" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle>Targets ({site.targets.length})</CardTitle>
              <CardDescription>
                Senha → OpenBao (secret_ref no inventário, sem secret no SQLite).
                Depois apply nos gateways. Também: {t('sites.syncGateways')}.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              {site.targets.length === 0 ? (
                <p className="text-sm text-muted-foreground">Nenhum target.</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Nome</TableHead>
                      <TableHead>Engine</TableHead>
                      <TableHead>Protocolo</TableHead>
                      <TableHead>Host:port</TableHead>
                      <TableHead>secret_ref</TableHead>
                      <TableHead>Roles</TableHead>
                      <TableHead />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {site.targets.map((t) => (
                      <TableRow key={t.nome}>
                        <TableCell className="font-medium">{t.nome}</TableCell>
                        <TableCell>{t.engine}</TableCell>
                        <TableCell>{t.protocolo}</TableCell>
                        <TableCell className="font-mono text-xs">
                          {t.host}:{t.port}
                        </TableCell>
                        <TableCell className="font-mono text-[10px] max-w-[140px] truncate">
                          {t.secret_ref ? (
                            <Badge variant="secondary">OpenBao</Badge>
                          ) : (
                            <span className="text-muted-foreground">—</span>
                          )}
                        </TableCell>
                        <TableCell className="text-xs">
                          {(t.roles || []).join(', ')}
                        </TableCell>
                        <TableCell className="text-right">
                          {(can('sites:update') ||
                            can('secrets:manage') ||
                            can('system:admin')) && (
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => {
                                setSecretTarget(t.nome)
                                setSecretUser(t.username || '')
                                setSecretPass('')
                              }}
                            >
                              <KeyRound className="h-3.5 w-3.5 mr-1" />
                              Secret
                            </Button>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
              {applyLog && (
                <pre className="rounded bg-muted p-3 text-xs whitespace-pre-wrap">
                  {applyLog}
                </pre>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      <Dialog
        open={!!secretTarget}
        onOpenChange={(o) => !o && setSecretTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Secret do target {secretTarget}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Grava em OpenBao (
            <span className="font-mono text-xs">
              secret/data/archgate/targets/…
            </span>
            ), associa secret_ref no site e aplica no gateway. A senha não fica
            no inventário.
          </p>
          <div className="space-y-2">
            <Label>Username (opcional)</Label>
            <Input
              className="font-mono"
              value={secretUser}
              onChange={(e) => setSecretUser(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label>Password (one-shot)</Label>
            <Input
              type="password"
              autoComplete="new-password"
              value={secretPass}
              onChange={(e) => setSecretPass(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSecretTarget(null)}>
              Cancelar
            </Button>
            <Button
              disabled={!secretPass || storeSecret.isPending}
              onClick={() => storeSecret.mutate()}
            >
              Gravar e apply
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
