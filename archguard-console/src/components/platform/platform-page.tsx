// Módulo Plataforma — saúde stack ArchGate + endpoints + runbooks (ADR-009)

import { useTranslation } from 'react-i18next'
import { useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  Activity,
  AlertTriangle,
  BookOpen,
  CheckCircle2,
  Cloud,
  ExternalLink,
  Gauge,
  Network,
  RefreshCw,
  Server,
  Shield,
  XCircle,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Input } from '@/components/ui/input'
import { PageHeader } from '@/components/shared/page-header'
import { PermissionGate } from '@/components/shared/permission-gate'
import {
  getPlatformOverviewFn,
  reconcileBrokerLeasesFn,
  migrateLegacyAccessGrantsFn,
  retryAuditOutboxFn,
  type PlatformService,
  type PlatformServiceStatus,
} from '@/server/platform-fn'

const STATUS_UI: Record<
  PlatformServiceStatus,
  {
    labelKey: string
    variant: 'default' | 'secondary' | 'destructive' | 'outline'
    icon: React.ComponentType<{ className?: string }>
  }
> = {
  ok: { labelKey: 'common.online', variant: 'default', icon: CheckCircle2 },
  degraded: {
    labelKey: 'dashboard.health.error',
    variant: 'secondary',
    icon: AlertTriangle,
  },
  error: {
    labelKey: 'dashboard.health.error',
    variant: 'destructive',
    icon: XCircle,
  },
  unreachable: {
    labelKey: 'common.unavailable',
    variant: 'destructive',
    icon: XCircle,
  },
  unconfigured: {
    labelKey: 'common.notConfigured',
    variant: 'outline',
    icon: AlertTriangle,
  },
}

const GROUP_LABEL_KEY: Record<PlatformService['group'], string> = {
  identity: 'nav.identity',
  gateway: 'nav.gateways',
  secrets: 'nav.secrets',
  connectivity: 'sites.connectivity',
  control_plane: 'nav.platform',
}

function ServiceRow({ svc }: { svc: PlatformService }) {
  const { t } = useTranslation()
  const ui = STATUS_UI[svc.status]
  const Icon = ui.icon
  return (
    <div className="flex flex-col gap-1 border-b py-3 last:border-0 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <p className="font-medium text-sm">{svc.name}</p>
          <Badge variant="outline" className="text-[10px]">
            {t(GROUP_LABEL_KEY[svc.group])}
          </Badge>
        </div>
        {svc.detail && (
          <p className="text-xs text-muted-foreground mt-0.5 break-all">
            {svc.detail}
          </p>
        )}
        {svc.endpoint && (
          <p className="text-[11px] font-mono text-muted-foreground mt-0.5 truncate">
            {svc.endpoint}
          </p>
        )}
      </div>
      <div className="flex items-center gap-2 shrink-0">
        {typeof svc.latency_ms === 'number' && (
          <span className="text-[11px] tabular-nums text-muted-foreground">
            {svc.latency_ms} ms
          </span>
        )}
        {svc.version && (
          <span className="text-[11px] font-mono text-muted-foreground">
            v{svc.version}
          </span>
        )}
        <Badge variant={ui.variant} className="gap-1">
          <Icon className="h-3 w-3" />
          {t(ui.labelKey)}
        </Badge>
      </div>
    </div>
  )
}

export function PlatformPage() {
  const { t } = useTranslation()
  const q = useQuery({
    queryKey: ['platform', 'overview'],
    queryFn: () => getPlatformOverviewFn(),
    refetchInterval: 30_000,
    staleTime: 10_000,
  })

  const data = q.data
  const [legacyPrincipal, setLegacyPrincipal] = useState('')
  const [legacyIdentityId, setLegacyIdentityId] = useState('')
  const retryOutbox = useMutation({
    mutationFn: () => retryAuditOutboxFn(),
    onSuccess: () => void q.refetch(),
  })
  const reconcileLeases = useMutation({
    mutationFn: () => reconcileBrokerLeasesFn(),
    onSuccess: () => void q.refetch(),
  })
  const migrateLegacy = useMutation({
    mutationFn: (dry_run: boolean) => migrateLegacyAccessGrantsFn({
      data: { principal: legacyPrincipal, identity_id: legacyIdentityId, dry_run },
    }),
    onSuccess: () => void q.refetch(),
  })

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title={
          <span className="inline-flex items-center gap-2">
            <Gauge className="h-7 w-7 text-primary" />
            {t('platform.title')}
          </span>
        }
        description={t('platform.subtitle')}
        actions={
          <Button
            variant="outline"
            size="sm"
            disabled={q.isFetching}
            onClick={() => void q.refetch()}
          >
            <RefreshCw
              className={`h-4 w-4 mr-2 ${q.isFetching ? 'animate-spin' : ''}`}
            />
            {t('common.refresh')}
          </Button>
        }
      />

      {q.isError && (
        <Card className="border-destructive">
          <CardContent className="pt-4 text-sm text-destructive flex gap-2">
            <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5" />
            {(q.error as Error).message}
          </CardContent>
        </Card>
      )}

      {/* Summary */}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        {q.isLoading || !data ? (
          Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-20 w-full" />
          ))
        ) : (
          <>
            <Card>
              <CardHeader className="pb-1">
                <CardTitle className="text-xs text-muted-foreground font-medium">
                  Serviços OK
                </CardTitle>
              </CardHeader>
              <CardContent className="text-2xl font-bold text-emerald-600">
                {data.summary.ok}/{data.summary.total}
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-1">
                <CardTitle className="text-xs text-muted-foreground font-medium">
                  Degradados
                </CardTitle>
              </CardHeader>
              <CardContent className="text-2xl font-bold">
                {data.summary.degraded}
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-1">
                <CardTitle className="text-xs text-muted-foreground font-medium">
                  Erro / offline
                </CardTitle>
              </CardHeader>
              <CardContent className="text-2xl font-bold text-destructive">
                {data.summary.error}
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-1">
                <CardTitle className="text-xs text-muted-foreground font-medium">
                  Sites inventário
                </CardTitle>
              </CardHeader>
              <CardContent className="text-2xl font-bold tabular-nums">
                {data.sites_count < 0 ? '—' : data.sites_count}
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-1">
                <CardTitle className="text-xs text-muted-foreground font-medium">
                  SoT / lab
                </CardTitle>
              </CardHeader>
              <CardContent className="flex flex-wrap gap-1.5">
                <Badge variant="outline" className="font-mono">
                  {data.sites_backend}
                </Badge>
                {data.lab && <Badge variant="secondary">ARCHGATE_LAB</Badge>}
              </CardContent>
            </Card>
          </>
        )}
      </div>

      {/* Inventory strip */}
      {data?.inventory && (
        <div className="grid gap-3 sm:grid-cols-3">
          <Card>
            <CardHeader className="pb-1">
              <CardTitle className="text-xs text-muted-foreground">
                Warpgate targets
              </CardTitle>
            </CardHeader>
            <CardContent className="text-xl font-bold tabular-nums">
              {data.inventory.warpgate_targets < 0
                ? '—'
                : data.inventory.warpgate_targets}
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-1">
              <CardTitle className="text-xs text-muted-foreground">
                Guacamole connections
              </CardTitle>
            </CardHeader>
            <CardContent className="text-xl font-bold tabular-nums">
              {data.inventory.guacamole_connections < 0
                ? '—'
                : data.inventory.guacamole_connections}
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-1">
              <CardTitle className="text-xs text-muted-foreground">
                Axis proprietários
              </CardTitle>
            </CardHeader>
            <CardContent className="text-xl font-bold tabular-nums">
              {data.inventory.axis_proprietarios < 0
                ? '—'
                : data.inventory.axis_proprietarios}
            </CardContent>
          </Card>
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-5">
        {/* Services */}
        <Card className="lg:col-span-3">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Activity className="h-4 w-4" />
              Saúde dos serviços
            </CardTitle>
            <CardDescription>
              Probes HTTP/DNS a partir do console (sem Docker socket). Atualiza
              a cada 30s.
              {data?.generated_at && (
                <span className="block font-mono text-[11px] mt-1">
                  gerado {new Date(data.generated_at).toLocaleString()}
                </span>
              )}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {q.isLoading || !data ? (
              <div className="space-y-3">
                {Array.from({ length: 6 }).map((_, i) => (
                  <Skeleton key={i} className="h-12 w-full" />
                ))}
              </div>
            ) : (
              data.services.map((svc) => <ServiceRow key={svc.id} svc={svc} />)
            )}
          </CardContent>
        </Card>

        {/* Endpoints + runbooks */}
        <div className="lg:col-span-2 space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Network className="h-4 w-4" />
                Endpoints
              </CardTitle>
              <CardDescription>
                Catálogo operacional (staging / lab).
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              {q.isLoading || !data ? (
                <Skeleton className="h-32 w-full" />
              ) : (
                Object.entries(data.endpoints).map(([k, v]) => (
                  <div key={k} className="flex flex-col gap-0.5">
                    <span className="text-xs text-muted-foreground">{k}</span>
                    <span className="font-mono text-xs break-all">
                      {v ?? '—'}
                    </span>
                  </div>
                ))
              )}
            </CardContent>
          </Card>

          {data && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <Activity className="h-4 w-4" />
                  Fila de auditoria
                </CardTitle>
                <CardDescription>Estado da entrega sem expor payloads ou erros sensíveis.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-2 text-sm">
                <div className="flex items-center justify-between"><span>Saúde</span><Badge variant={data.audit_outbox.health === 'healthy' ? 'default' : data.audit_outbox.health === 'blocked' ? 'destructive' : 'secondary'}>{data.audit_outbox.health}</Badge></div>
                <div className="flex justify-between"><span>Pendentes</span><strong>{data.audit_outbox.pending}</strong></div>
                <div className="flex justify-between"><span>Em publicação</span><strong>{data.audit_outbox.publishing}</strong></div>
                <div className="flex justify-between"><span>Falhas</span><strong className={data.audit_outbox.failed ? 'text-destructive' : ''}>{data.audit_outbox.failed}</strong></div>
                <div className="flex justify-between"><span>Claims presos</span><strong className={data.audit_outbox.stale_claims ? 'text-destructive' : ''}>{data.audit_outbox.stale_claims}</strong></div>
                <div className="flex justify-between"><span>Publicados</span><strong>{data.audit_outbox.published}</strong></div>
                {data.audit_outbox.oldest_pending_at && <p className="text-xs text-muted-foreground">Mais antigo: {new Date(data.audit_outbox.oldest_pending_at).toLocaleString()} ({data.audit_outbox.oldest_pending_age_seconds ?? 0}s)</p>}
                <Button size="sm" variant="outline" disabled={retryOutbox.isPending} onClick={() => retryOutbox.mutate()}>
                  {retryOutbox.isPending ? 'Tentando…' : 'Tentar entrega agora'}
                </Button>
                {retryOutbox.isError && <p className="text-xs text-destructive">{(retryOutbox.error as Error).message}</p>}
              </CardContent>
            </Card>
          )}

          {data && data.legacy_grants.total > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <AlertTriangle className="h-4 w-4 text-amber-600" />
                  Grants legados
                </CardTitle>
                <CardDescription>Concessões sem identidade canônica aguardando migração controlada.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-2 text-sm">
                <div className="flex justify-between"><span>Grants</span><strong>{data.legacy_grants.total}</strong></div>
                <div className="flex justify-between"><span>Principais afetados</span><strong>{data.legacy_grants.principals}</strong></div>
                {data.legacy_grants.oldest_created_at && <p className="text-xs text-muted-foreground">Mais antigo: {new Date(data.legacy_grants.oldest_created_at).toLocaleString()}</p>}
                <p className="text-xs text-amber-700">Não atribua identidade automaticamente: confirme o vínculo no control plane antes de migrar.</p>
                <PermissionGate require={['settings:update', 'system:admin']} any>
                  <div className="grid gap-2 border-t pt-3">
                    <Input placeholder="Principal legado" value={legacyPrincipal} onChange={(e) => setLegacyPrincipal(e.target.value)} />
                    <Input placeholder="identity_id canônico confirmado" value={legacyIdentityId} onChange={(e) => setLegacyIdentityId(e.target.value)} />
                    <div className="flex flex-wrap gap-2">
                      <Button size="sm" variant="outline" disabled={migrateLegacy.isPending || !legacyPrincipal || !legacyIdentityId} onClick={() => migrateLegacy.mutate(true)}>
                        Prévia
                      </Button>
                      <Button size="sm" disabled={migrateLegacy.isPending || !legacyPrincipal || !legacyIdentityId} onClick={() => {
                        if (window.confirm('Confirma vincular todos os grants legados deste principal à identidade informada?')) migrateLegacy.mutate(false)
                      }}>
                        {migrateLegacy.isPending ? 'Processando…' : 'Aplicar vínculo'}
                      </Button>
                    </div>
                    {migrateLegacy.data && <p className="text-xs text-muted-foreground">{migrateLegacy.data.dry_run ? `Prévia: ${migrateLegacy.data.affected} grant(s) serão atualizados.` : `${migrateLegacy.data.affected} grant(s) atualizados.`}</p>}
                    {migrateLegacy.isError && <p className="text-xs text-destructive">{(migrateLegacy.error as Error).message}</p>}
                  </div>
                </PermissionGate>
              </CardContent>
            </Card>
          )}

          {data && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <Shield className="h-4 w-4" />
                  Reconciliação de leases
                </CardTitle>
                <CardDescription>Somente sessões e leases emitidos pelo console.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-2 text-sm">
                <div className="flex items-center justify-between"><span>Agendador</span><Badge variant={data.broker_reconciler.enabled ? 'default' : 'outline'}>{data.broker_reconciler.enabled ? 'habilitado' : 'desabilitado'}</Badge></div>
                {data.broker_reconciler.interval_ms && <div className="flex justify-between"><span>Intervalo</span><strong>{Math.round(data.broker_reconciler.interval_ms / 1000)}s</strong></div>}
                {data.broker_reconciler.last_run_at ? <p className="text-xs text-muted-foreground">Último ciclo: {new Date(data.broker_reconciler.last_run_at).toLocaleString()}</p> : <p className="text-xs text-muted-foreground">Nenhum ciclo executado neste processo.</p>}
                {data.broker_reconciler.last_result && <p className={data.broker_reconciler.last_result.failed ? 'text-xs text-destructive' : 'text-xs text-muted-foreground'}>Tentativas: {data.broker_reconciler.last_result.attempted} · encerradas: {data.broker_reconciler.last_result.closed} · falhas: {data.broker_reconciler.last_result.failed}</p>}
                {data.broker_reconciler.last_error && <p className="text-xs text-destructive">Último ciclo falhou; consulte os logs do processo.</p>}
                <PermissionGate require={['settings:update', 'system:admin']} any>
                  <Button size="sm" variant="outline" disabled={reconcileLeases.isPending} onClick={() => reconcileLeases.mutate()}>
                    {reconcileLeases.isPending ? 'Executando…' : 'Executar agora'}
                  </Button>
                  {reconcileLeases.isError && <p className="text-xs text-destructive">Reconciliação falhou; consulte os logs do processo.</p>}
                </PermissionGate>
              </CardContent>
            </Card>
          )}

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <BookOpen className="h-4 w-4" />
                Runbooks & módulos
              </CardTitle>
            </CardHeader>
            <CardContent className="grid gap-2">
              {(data?.runbooks || []).map((rb) => {
                const inner = (
                  <>
                    <div className="min-w-0">
                      <p className="text-sm font-medium">{rb.title}</p>
                      <p className="text-xs text-muted-foreground">
                        {rb.description}
                      </p>
                    </div>
                    {rb.external || rb.href.startsWith('http') ? (
                      <ExternalLink className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    ) : (
                      <Server className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    )}
                  </>
                )
                if (rb.href.startsWith('http')) {
                  return (
                    <a
                      key={rb.id}
                      href={rb.href}
                      target="_blank"
                      rel="noreferrer"
                      className="flex items-center justify-between gap-2 rounded-md border px-3 py-2 hover:bg-muted/50 transition-colors"
                    >
                      {inner}
                    </a>
                  )
                }
                return (
                  <Link
                    key={rb.id}
                    to={rb.href}
                    className="flex items-center justify-between gap-2 rounded-md border px-3 py-2 hover:bg-muted/50 transition-colors"
                  >
                    {inner}
                  </Link>
                )
              })}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Shield className="h-4 w-4" />
                Atalhos control plane
              </CardTitle>
            </CardHeader>
            <CardContent className="grid gap-2 sm:grid-cols-2">
              <Button asChild variant="outline" size="sm" className="justify-start">
                <Link to="/sites">
                  <Cloud className="h-4 w-4 mr-2" /> Sites
                </Link>
              </Button>
              <Button asChild variant="outline" size="sm" className="justify-start">
                <Link to="/gateways">
                  <Server className="h-4 w-4 mr-2" /> Gateways
                </Link>
              </Button>
              <Button asChild variant="outline" size="sm" className="justify-start">
                <Link to="/secrets">
                  <Shield className="h-4 w-4 mr-2" /> Segredos
                </Link>
              </Button>
              <Button asChild variant="outline" size="sm" className="justify-start">
                <Link to="/integrations/mentors-axis">
                  <Cloud className="h-4 w-4 mr-2" /> Axis
                </Link>
              </Button>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
