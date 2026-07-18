// Módulo Plataforma — saúde stack ArchGate + endpoints + runbooks (ADR-009)

import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
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
import {
  getPlatformOverviewFn,
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

  return (
    <div className="space-y-6 p-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <Gauge className="h-7 w-7 text-primary" />
            {t('platform.title')}
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            {t('platform.subtitle')}
          </p>
        </div>
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
      </div>

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
