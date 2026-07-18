// CP-7 — Mentors Axis sync (proprietários → sites + Kanidm + Warpgate roles)

import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { RefreshCw, Building2, Cloud, GitBranch } from 'lucide-react'
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
  getMentorsAxisStatusFn,
  listMentorsAxisProprietariosFn,
  syncMentorsAxisTenantsFn,
} from '@/server/mentors-axis-fn'
import { usePermissions } from '@/lib/hooks/use-permissions'
import { siteKeys } from '@/lib/hooks/use-sites'

export function MentorsAxisPage() {
  const { can } = usePermissions()
  const canSync =
    can('sites:update') || can('sites:create') || can('system:admin')
  const qc = useQueryClient()
  const [showRaw, setShowRaw] = useState(false)

  const status = useQuery({
    queryKey: ['mentors-axis', 'status'],
    queryFn: () => getMentorsAxisStatusFn(),
  })

  const list = useQuery({
    queryKey: ['mentors-axis', 'proprietarios'],
    queryFn: () => listMentorsAxisProprietariosFn(),
    enabled: canSync && !!status.data?.configured,
  })

  const sync = useMutation({
    mutationFn: () => syncMentorsAxisTenantsFn(),
    onSuccess: (res) => {
      if (res.orchestration_ok) {
        toast.success(
          `Orquestração OK: ${res.created} criados, ${res.updated} atualizados · Kanidm +${res.kanidm_created} · WG roles ${res.warpgate_roles_ok}`,
        )
      } else {
        toast.error(
          `Sync parcial: ${res.steps_failed} passo(s) falharam — veja tabela`,
        )
      }
      void qc.invalidateQueries({ queryKey: ['mentors-axis'] })
      void qc.invalidateQueries({ queryKey: siteKeys.all })
      void qc.invalidateQueries({ queryKey: ['warpgate'] })
      void qc.invalidateQueries({ queryKey: ['platform'] })
    },
    onError: (e) => toast.error((e as Error).message),
  })

  return (
    <div className="space-y-6 p-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <Cloud className="h-7 w-7 text-primary" />
            Mentors Axis
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            SoT comercial de clientes. Sync orquestra:{' '}
            <strong>site</strong> → <strong>grupo Kanidm tenant_*</strong> →{' '}
            <strong>role Warpgate</strong> (ADR-004). Pessoas/offboarding full =
            Fase 4.
          </p>
        </div>
        {canSync && status.data?.configured && (
          <Button onClick={() => sync.mutate()} disabled={sync.isPending}>
            <RefreshCw
              className={`h-4 w-4 mr-2 ${sync.isPending ? 'animate-spin' : ''}`}
            />
            Sincronizar + orquestrar
          </Button>
        )}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Status da integração</CardTitle>
          <CardDescription>
            Spring lab: <code className="text-xs">archgate-axis-spring:9030</code>{' '}
            · host <code className="text-xs">:9031</code>
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {status.isLoading ? (
            <Skeleton className="h-6 w-40" />
          ) : (
            <>
              <div className="flex flex-wrap gap-2">
                <Badge
                  variant={status.data?.configured ? 'default' : 'destructive'}
                >
                  {status.data?.mode || 'disabled'}
                </Badge>
                {status.data?.auth && (
                  <Badge variant="outline">auth: {status.data.auth}</Badge>
                )}
                {status.data?.kanidm_ensure_groups && (
                  <Badge variant="secondary">Kanidm groups</Badge>
                )}
                {status.data?.warpgate_ensure_roles && (
                  <Badge variant="secondary">Warpgate roles</Badge>
                )}
                {status.data?.url && (
                  <span className="text-xs font-mono text-muted-foreground">
                    {status.data.url}
                  </span>
                )}
              </div>
              {status.data?.orchestration && (
                <div className="flex flex-wrap gap-1.5 items-center text-xs text-muted-foreground">
                  <GitBranch className="h-3.5 w-3.5" />
                  pipeline:{' '}
                  {status.data.orchestration.map((s) => (
                    <Badge key={s} variant="outline" className="font-mono text-[10px]">
                      {s}
                    </Badge>
                  ))}
                </div>
              )}
              {status.data?.endpoints && (
                <pre className="text-xs rounded bg-muted p-2 overflow-auto">
                  {`login: ${status.data.endpoints.login}
list:  ${status.data.endpoints.list}
tenant header X-TENANT-IDs: ${status.data.tenant_id || '—'}`}
                </pre>
              )}
            </>
          )}
        </CardContent>
      </Card>

      {canSync && status.data?.configured && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Building2 className="h-5 w-5" />
              Proprietários (preview)
            </CardTitle>
          </CardHeader>
          <CardContent>
            {list.isLoading ? (
              <Skeleton className="h-24 w-full" />
            ) : list.isError ? (
              <p className="text-sm text-destructive">
                {(list.error as Error).message}
              </p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Descrição</TableHead>
                    <TableHead>Slug / tenant / role</TableHead>
                    <TableHead>Admin Axis</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Site local</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(list.data || []).map((p) => (
                    <TableRow key={p.id}>
                      <TableCell className="font-medium">
                        {p.descricao || p.id}
                        {p.cnpj && (
                          <div className="text-[11px] text-muted-foreground font-mono">
                            {p.cnpj}
                          </div>
                        )}
                      </TableCell>
                      <TableCell className="font-mono text-xs">
                        <div>{p.slug_sugerido}</div>
                        <div className="text-muted-foreground">
                          {p.tenant_group}
                        </div>
                        <div className="text-muted-foreground">
                          {p.warpgate_role}
                        </div>
                      </TableCell>
                      <TableCell className="text-xs">
                        {p.admin_email || '—'}
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline">{p.status || '—'}</Badge>
                      </TableCell>
                      <TableCell>
                        {p.site_exists ? (
                          <Link
                            to="/sites/$slug"
                            params={{ slug: p.slug_sugerido }}
                            className="text-primary text-sm underline"
                          >
                            existe
                          </Link>
                        ) : (
                          <span className="text-xs text-muted-foreground">
                            será criado
                          </span>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      )}

      {sync.data && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <div>
              <CardTitle>Última orquestração</CardTitle>
              <CardDescription>
                mode={sync.data.mode} · created={sync.data.created} · updated=
                {sync.data.updated} · kanidm+{sync.data.kanidm_created} · wg
                roles {sync.data.warpgate_roles_ok} · failed steps=
                {sync.data.steps_failed}
              </CardDescription>
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowRaw((v) => !v)}
            >
              {showRaw ? 'Tabela' : 'JSON'}
            </Button>
          </CardHeader>
          <CardContent>
            {showRaw ? (
              <pre className="text-xs rounded bg-muted p-3 overflow-auto max-h-96">
                {JSON.stringify(sync.data, null, 2)}
              </pre>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Cliente</TableHead>
                    <TableHead>Site</TableHead>
                    <TableHead>Kanidm</TableHead>
                    <TableHead>Warpgate</TableHead>
                    <TableHead>Admin</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sync.data.results.map((r) => (
                    <TableRow key={r.slug}>
                      <TableCell className="font-medium text-sm">
                        {r.cliente}
                        <Badge variant="outline" className="ml-2 text-[10px]">
                          {r.action}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Link
                          to="/sites/$slug"
                          params={{ slug: r.slug }}
                          className="font-mono text-xs text-primary underline"
                        >
                          {r.slug}
                        </Link>
                      </TableCell>
                      <TableCell className="font-mono text-[11px]">
                        {r.kanidm_group || '—'}
                      </TableCell>
                      <TableCell className="font-mono text-[11px]">
                        {r.warpgate_role || '—'}
                      </TableCell>
                      <TableCell className="text-xs">
                        {r.admin_email || '—'}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
