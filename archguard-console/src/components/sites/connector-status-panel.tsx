// CP-5 — connector checklist + deploy hints for a site

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Cable,
  CheckCircle2,
  Circle,
  Copy,
  AlertTriangle,
  RefreshCw,
} from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { Skeleton } from '@/components/ui/skeleton'
import {
  getConnectorStatusFn,
  updateConnectorChecklistFn,
} from '@/server/connector-fn'
import { usePermissions } from '@/lib/hooks/use-permissions'
import { siteKeys } from '@/lib/hooks/use-sites'

export function ConnectorStatusPanel({ slug }: { slug: string }) {
  const { can } = usePermissions()
  const canWrite = can('sites:update') || can('system:admin')
  const qc = useQueryClient()

  const q = useQuery({
    queryKey: ['connector', slug],
    queryFn: () => getConnectorStatusFn({ data: { slug } }),
  })

  const toggle = useMutation({
    mutationFn: (args: { item_id: string; done: boolean }) =>
      updateConnectorChecklistFn({
        data: { slug, item_id: args.item_id, done: args.done },
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['connector', slug] })
      void qc.invalidateQueries({ queryKey: siteKeys.detail(slug) })
      void qc.invalidateQueries({ queryKey: siteKeys.all })
    },
    onError: (e) => toast.error((e as Error).message),
  })

  if (q.isLoading) {
    return <Skeleton className="h-48 w-full" />
  }
  if (q.isError || !q.data) {
    return (
      <p className="text-sm text-destructive">
        {(q.error as Error)?.message || 'Falha ao carregar connector'}
      </p>
    )
  }

  const { progress, items, hints, risk, probes, site } = q.data

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-2">
          <div>
            <CardTitle className="flex items-center gap-2">
              <Cable className="h-5 w-5" />
              Connector — checklist
            </CardTitle>
            <CardDescription>
              Path institucional ({site.stack} / {site.tipo}) ·{' '}
              <span className="font-mono">{site.connector_id || '—'}</span>
            </CardDescription>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => void q.refetch()}
          >
            <RefreshCw className="h-4 w-4 mr-1" /> Atualizar
          </Button>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap items-center gap-2">
            <Badge
              variant={
                risk === 'bad'
                  ? 'destructive'
                  : risk === 'warn'
                    ? 'secondary'
                    : 'default'
              }
            >
              {risk === 'bad'
                ? 'vpn_user_exception'
                : risk === 'warn'
                  ? 'tipo a confirmar'
                  : 'tipo institucional'}
            </Badge>
            <span className="text-sm text-muted-foreground">
              {progress.done}/{progress.total} ({progress.pct}%)
            </span>
          </div>
          <Progress value={progress.pct} className="h-2" />

          <ul className="space-y-3">
            {items.map((it) => (
              <li
                key={it.id}
                className="flex items-start gap-3 rounded-lg border p-3"
              >
                {it.done ? (
                  <CheckCircle2 className="h-5 w-5 text-emerald-600 shrink-0 mt-0.5" />
                ) : (
                  <Circle className="h-5 w-5 text-muted-foreground shrink-0 mt-0.5" />
                )}
                <div className="flex-1 min-w-0 space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium text-sm">{it.label}</span>
                    <Badge variant="outline" className="text-[10px]">
                      {it.source}
                    </Badge>
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {it.description}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    Como: {it.how}
                  </p>
                  {it.detail && (
                    <p className="text-xs font-mono truncate">{it.detail}</p>
                  )}
                </div>
                {canWrite && (
                  <Switch
                    checked={it.done}
                    disabled={toggle.isPending && it.source === 'auto'}
                    onCheckedChange={(done) =>
                      toggle.mutate({ item_id: it.id, done })
                    }
                    aria-label={it.label}
                  />
                )}
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle className="text-base">Comandos de deploy</CardTitle>
            <CardDescription>
              Referência no runtime — não executa do browser.
            </CardDescription>
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              void navigator.clipboard.writeText(hints.join('\n'))
              toast.success('Comandos copiados')
            }}
          >
            <Copy className="h-4 w-4 mr-1" /> Copiar
          </Button>
        </CardHeader>
        <CardContent>
          <pre className="rounded bg-muted p-3 text-xs overflow-auto whitespace-pre-wrap">
            {hints.join('\n')}
          </pre>
        </CardContent>
      </Card>

      {probes.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Probes lab (best-effort)</CardTitle>
            <CardDescription>
              DNS/TCP a partir do container do console — só informativo.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {probes.map((p) => (
              <div
                key={p.name}
                className="flex items-start gap-2 text-xs font-mono"
              >
                {p.ok ? (
                  <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600 shrink-0 mt-0.5" />
                ) : (
                  <AlertTriangle className="h-3.5 w-3.5 text-amber-600 shrink-0 mt-0.5" />
                )}
                <span>
                  {p.name}: {p.detail}
                </span>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
