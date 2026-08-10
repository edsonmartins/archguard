// Admin-first connector control — materialize via host agent (not day-2 SSH)

import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Cable,
  CheckCircle2,
  Circle,
  Copy,
  AlertTriangle,
  RefreshCw,
  Play,
  Square,
  Radar,
} from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  deployConnectorFn,
  getConnectorStatusFn,
  probeConnectorFn,
  planConnectorUpgradeFn,
  getConnectorUpgradePlansFn,
  stopConnectorFn,
  updateConnectorChecklistFn,
} from '@/server/connector-fn'
import { usePermissions } from '@/lib/hooks/use-permissions'
import { siteKeys } from '@/lib/hooks/use-sites'

export function ConnectorStatusPanel({ slug }: { slug: string }) {
  const { can } = usePermissions()
  const canWrite = can('sites:update') || can('system:admin')
  const qc = useQueryClient()

  const [connectorId, setConnectorId] = useState('')
  const [stack, setStack] = useState<'openfortivpn' | 'openvpn'>('openfortivpn')
  const [fortiHost, setFortiHost] = useState('')
  const [fortiUser, setFortiUser] = useState('')
  const [fortiPass, setFortiPass] = useState('')
  const [rawConfig, setRawConfig] = useState('')
  const [probeHost, setProbeHost] = useState('')
  const [probePort, setProbePort] = useState('22')
  const [upgradeVersion, setUpgradeVersion] = useState('')
  const [upgradeUrl, setUpgradeUrl] = useState('')
  const [upgradeSha256, setUpgradeSha256] = useState('')

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

  const deploy = useMutation({
    mutationFn: async () => {
      const id =
        connectorId.trim() ||
        q.data?.site.connector_id ||
        `connector-${slug}`
      if (stack === 'openvpn') {
        if (!rawConfig.trim()) {
          throw new Error('Cole o conteúdo do .ovpn no campo config')
        }
        return deployConnectorFn({
          data: {
            slug,
            connector_id: id,
            stack,
            config: rawConfig,
            start: true,
          },
        })
      }
      if (rawConfig.trim()) {
        return deployConnectorFn({
          data: {
            slug,
            connector_id: id,
            stack,
            config: rawConfig,
            start: true,
          },
        })
      }
      return deployConnectorFn({
        data: {
          slug,
          connector_id: id,
          stack,
          forti: {
            host: fortiHost,
            username: fortiUser,
            password: fortiPass,
          },
          start: true,
        },
      })
    },
    onSuccess: (res) => {
      const r = res as { connector_id: string; started: boolean }
      toast.success(
        `Connector ${r.connector_id} materializado${r.started ? ' e iniciado' : ''}`,
      )
      setFortiPass('')
      void qc.invalidateQueries({ queryKey: ['connector', slug] })
      void qc.invalidateQueries({ queryKey: siteKeys.detail(slug) })
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const stop = useMutation({
    mutationFn: () => {
      const id =
        connectorId.trim() ||
        q.data?.site.connector_id ||
        `connector-${slug}`
      return stopConnectorFn({
        data: { slug, connector_id: id, stack },
      })
    },
    onSuccess: () => {
      toast.success('Connector parado')
      void qc.invalidateQueries({ queryKey: ['connector', slug] })
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const probe = useMutation({
    mutationFn: () =>
      probeConnectorFn({
        data: {
          slug,
          host: probeHost.trim(),
          port: Number(probePort) || 22,
        },
      }),
    onSuccess: (r) => {
      if (r.ok) toast.success(`Alcance OK ${r.host}:${r.port}`)
      else toast.error(`Sem alcance: ${r.detail || 'fail'}`)
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const upgradePlan = useMutation({
    mutationFn: () => planConnectorUpgradeFn({
      data: {
        slug,
        version: upgradeVersion.trim(),
        url: upgradeUrl.trim(),
        sha256: upgradeSha256.trim(),
      },
    }),
    onSuccess: (result) => {
      toast.success(`Plano de upgrade validado: ${(result.plan as { upgrade?: { action?: string } })?.upgrade?.action || 'pending_approval'}`)
      void qc.invalidateQueries({ queryKey: ['connector-upgrade-plans', slug] })
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

  const { progress, items, hints, risk, probes, site, runtime } = q.data

  // Prefill connector id once
  if (!connectorId && site.connector_id) {
    // avoid setState during render — only default in UI value
  }
  const effectiveId =
    connectorId || site.connector_id || `connector-${slug}`

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-2">
          <div>
            <CardTitle className="flex items-center gap-2">
              <Cable className="h-5 w-5" />
              Connector — admin
            </CardTitle>
            <CardDescription>
              Happy path: materializar VPN pelo console (agent no host). Scripts
              SSH só bootstrap/break-glass.
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
        <CardContent className="space-y-3">
          <div className="flex flex-wrap gap-2 items-center">
            <Badge
              variant={
                runtime?.agent_ok
                  ? 'default'
                  : runtime?.agent_configured
                    ? 'destructive'
                    : 'secondary'
              }
            >
              {runtime?.agent_ok
                ? 'agent online'
                : runtime?.agent_configured
                  ? 'agent offline'
                  : 'agent não configurado'}
            </Badge>
            {runtime?.agent_error && (
              <span className="text-xs text-destructive">
                {runtime.agent_error}
              </span>
            )}
          </div>
          {runtime?.connectors && runtime.connectors.length > 0 && (
            <div className="rounded border p-2 text-xs font-mono space-y-1">
              {runtime.connectors.map((c) => (
                <div key={c.id}>
                  {c.id} · {c.stack} · conf=
                  {c.conf_present ? 'yes' : 'no'} · unit=
                  {c.unit?.state || '—'}
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {canWrite && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Materializar connector</CardTitle>
            <CardDescription>
              Secret é one-shot (não grava no inventário). Forti: host/user/senha
              ou conf completa. OpenVPN: cole o .ovpn.
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1">
              <Label>Connector id</Label>
              <Input
                className="font-mono"
                value={connectorId || site.connector_id || ''}
                onChange={(e) => setConnectorId(e.target.value)}
                placeholder={effectiveId}
              />
            </div>
            <div className="space-y-1">
              <Label>Stack</Label>
              <Select
                value={stack}
                onValueChange={(v) => setStack(v as 'openfortivpn' | 'openvpn')}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="openfortivpn">openfortivpn (Forti)</SelectItem>
                  <SelectItem value="openvpn">openvpn</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {stack === 'openfortivpn' && (
              <>
                <div className="space-y-1">
                  <Label>Forti host</Label>
                  <Input
                    value={fortiHost}
                    onChange={(e) => setFortiHost(e.target.value)}
                    placeholder="vpn.cliente.example"
                  />
                </div>
                <div className="space-y-1">
                  <Label>Usuário serviço</Label>
                  <Input
                    value={fortiUser}
                    onChange={(e) => setFortiUser(e.target.value)}
                    placeholder="svc_archgate"
                  />
                </div>
                <div className="space-y-1 sm:col-span-2">
                  <Label>Senha (one-shot)</Label>
                  <Input
                    type="password"
                    value={fortiPass}
                    onChange={(e) => setFortiPass(e.target.value)}
                    autoComplete="new-password"
                  />
                </div>
              </>
            )}
            <div className="space-y-1 sm:col-span-2">
              <Label>
                {stack === 'openvpn'
                  ? 'client.ovpn (conteúdo completo)'
                  : 'Ou conf completa (opcional, substitui campos)'}
              </Label>
              <Textarea
                className="font-mono text-xs min-h-[120px]"
                value={rawConfig}
                onChange={(e) => setRawConfig(e.target.value)}
                placeholder={
                  stack === 'openvpn'
                    ? 'client\ndev tun\n…'
                    : 'host = …\npassword = …'
                }
              />
            </div>
            <div className="flex flex-wrap gap-2 sm:col-span-2">
              <Button
                disabled={deploy.isPending || !runtime?.agent_ok}
                onClick={() => deploy.mutate()}
              >
                <Play className="h-4 w-4 mr-1" />
                Materializar e iniciar
              </Button>
              <Button
                variant="outline"
                disabled={stop.isPending || !runtime?.agent_ok}
                onClick={() => stop.mutate()}
              >
                <Square className="h-4 w-4 mr-1" />
                Parar
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {canWrite && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base flex items-center gap-2">
              <Radar className="h-4 w-4" /> Testar alcance
            </CardTitle>
            <CardDescription>
              Probe TCP a partir do host (via agent) — IP interno do target.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-wrap gap-2 items-end">
            <div className="space-y-1">
              <Label>Host</Label>
              <Input
                className="font-mono w-48"
                value={probeHost}
                onChange={(e) => setProbeHost(e.target.value)}
                placeholder="10.x.x.x"
              />
            </div>
            <div className="space-y-1">
              <Label>Porta</Label>
              <Input
                className="w-24"
                value={probePort}
                onChange={(e) => setProbePort(e.target.value)}
              />
            </div>
            <Button
              variant="secondary"
              disabled={probe.isPending || !probeHost || !runtime?.agent_ok}
              onClick={() => probe.mutate()}
            >
              Probe
            </Button>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-2">
          <div>
            <CardTitle className="text-base">Checklist</CardTitle>
            <CardDescription>
              Path institucional ({site.stack} / {site.tipo}) ·{' '}
              <span className="font-mono">{site.connector_id || '—'}</span>
            </CardDescription>
          </div>
          <Badge
            variant={
              risk === 'bad'
                ? 'destructive'
                : risk === 'warn'
                  ? 'secondary'
                  : 'default'
            }
          >
            {progress.done}/{progress.total}
          </Badge>
        </CardHeader>
        <CardContent className="space-y-4">
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
                  <span className="font-medium text-sm">{it.label}</span>
                  <p className="text-xs text-muted-foreground">{it.description}</p>
                </div>
                {canWrite && (
                  <Switch
                    checked={it.done}
                    disabled={toggle.isPending}
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
            <CardTitle className="text-base">Break-glass (referência)</CardTitle>
            <CardDescription>
              Só se o agent estiver fora — preferir materializar acima.
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
            <CardTitle className="text-base">Probes lab</CardTitle>
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
      <ConnectorInventoryCard />
      {canWrite && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Planejar upgrade</CardTitle>
            <CardDescription>
              Valida o manifesto no agent. Nenhum arquivo é baixado ou aplicado nesta etapa.
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1">
              <Label>Versão alvo</Label>
              <Input value={upgradeVersion} onChange={(e) => setUpgradeVersion(e.target.value)} placeholder="1.1.0" />
            </div>
            <div className="space-y-1">
              <Label>URL HTTPS do artefato</Label>
              <Input value={upgradeUrl} onChange={(e) => setUpgradeUrl(e.target.value)} placeholder="https://.../connector" />
            </div>
            <div className="space-y-1 sm:col-span-2">
              <Label>SHA-256</Label>
              <Input className="font-mono" value={upgradeSha256} onChange={(e) => setUpgradeSha256(e.target.value)} placeholder="64 caracteres hexadecimais" />
            </div>
            <div className="sm:col-span-2">
              <Button
                variant="outline"
                disabled={upgradePlan.isPending || !upgradeVersion.trim() || !upgradeUrl.trim() || upgradeSha256.trim().length !== 64}
                onClick={() => upgradePlan.mutate()}
              >
                {upgradePlan.isPending ? 'Validando…' : 'Validar plano'}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
      <UpgradePlanHistory slug={slug} />
    </div>
  )
}

function UpgradePlanHistory({ slug }: { slug: string }) {
  const q = useQuery({
    queryKey: ['connector-upgrade-plans', slug],
    queryFn: () => getConnectorUpgradePlansFn({ data: { slug } }),
    staleTime: 15_000,
  })
  const plans = q.data?.plans || []

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Histórico de planos</CardTitle>
        <CardDescription>Últimos 25 planos registrados para este site.</CardDescription>
      </CardHeader>
      <CardContent>
        {q.isLoading && <Skeleton className="h-16 w-full" />}
        {q.isError && <p className="text-sm text-destructive">{(q.error as Error).message}</p>}
        {!q.isLoading && !q.isError && plans.length === 0 && (
          <p className="text-sm text-muted-foreground">Nenhum plano registrado.</p>
        )}
        {plans.length > 0 && (
          <div className="space-y-2">
            {plans.map((plan) => (
              <div key={plan.id} className="rounded border p-3 text-sm space-y-1">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="font-medium">{plan.version}</span>
                  <Badge variant={plan.status === 'noop' ? 'secondary' : 'outline'}>{plan.status}</Badge>
                </div>
                <div className="text-xs text-muted-foreground break-all">
                  SHA-256: <span className="font-mono">{plan.sha256}</span>
                </div>
                <div className="text-xs text-muted-foreground break-all">Artefato: {plan.artifact_url}</div>
                <div className="text-xs text-muted-foreground">
                  {new Date(plan.created_at).toLocaleString()} · {plan.created_by}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function ConnectorInventoryCard() {
  const q = useQuery({
    queryKey: ['connector-inventory'],
    queryFn: async () => {
      const responses = await Promise.all([
        fetch('/api/org/v1/connectors/inventory'),
        fetch('/api/org/v1/connectors/diagnostics'),
        fetch('/api/org/v1/connectors/inventory/history'),
      ])
      const failed = responses.find((res) => !res.ok)
      if (failed) throw new Error(`Diagnóstico indisponível (${failed.status})`)
      const [inventory, diagnostics, history] = await Promise.all(responses.map((res) => res.json())) as [
        { connectors: Array<{
          connector_id: string
          status: string
          agent_version: string
          capabilities: string[]
          last_seen_at: string
          inventory: {
            os?: string
            os_release?: string
            architecture?: string
            hostname?: string
            interfaces?: string[]
          }
        }> },
        { diagnostics: Array<{
          connector_id: string
          state: string
          age_seconds: number
          certificates: Array<{ serial_suffix: string; status: string }>
        }> },
        { history: Array<{ connector_id: string; observed_at: string }> },
      ]
      return { ...inventory, diagnostics: diagnostics.diagnostics, history: history.history }
    },
    staleTime: 15_000,
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Inventário runtime</CardTitle>
        <CardDescription>
          Último heartbeat recebido; nenhum segredo ou conteúdo de alvo é exibido.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {q.isLoading && <Skeleton className="h-20 w-full" />}
        {q.isError && <p className="text-sm text-muted-foreground">{(q.error as Error).message}</p>}
        {!q.isLoading && !q.isError && q.data?.connectors.length === 0 && (
          <p className="text-sm text-muted-foreground">Nenhum heartbeat registrado.</p>
        )}
        {q.data?.connectors.map((connector) => (
          <div key={connector.connector_id} className="rounded border p-3 space-y-2 text-sm">
            {(() => {
              const diagnostic = q.data?.diagnostics.find((item) => item.connector_id === connector.connector_id)
              const snapshots = q.data?.history.filter((item) => item.connector_id === connector.connector_id) || []
              return (
                <>
            <div className="flex items-center justify-between gap-2">
              <span className="font-medium">{connector.connector_id}</span>
              <Badge variant={connector.status === 'ready' ? 'default' : 'destructive'}>{connector.status}</Badge>
            </div>
            <div className="grid gap-1 sm:grid-cols-2 text-xs text-muted-foreground">
              <span>Agent: {connector.agent_version}</span>
              <span>Último heartbeat: {new Date(connector.last_seen_at).toLocaleString()}</span>
              <span>Host: {connector.inventory.hostname || '—'}</span>
              <span>SO: {[connector.inventory.os, connector.inventory.os_release].filter(Boolean).join(' ') || '—'}</span>
              <span>Arquitetura: {connector.inventory.architecture || '—'}</span>
              <span>Interfaces: {connector.inventory.interfaces?.join(', ') || '—'}</span>
            </div>
            <div className="flex flex-wrap gap-1">
              {connector.capabilities.map((capability) => <Badge key={capability} variant="outline">{capability}</Badge>)}
            </div>
            {diagnostic && (
              <div className="border-t pt-2 text-xs text-muted-foreground">
                <div className="flex flex-wrap gap-x-4 gap-y-1">
                  <span>Diagnóstico: <strong>{diagnostic.state}</strong> ({diagnostic.age_seconds}s)</span>
                  <span>Snapshots: {snapshots.length}</span>
                  <span>Certificados: {diagnostic.certificates.filter((certificate) => certificate.status === 'active').length} ativos</span>
                </div>
                {diagnostic.certificates.length > 0 && (
                  <div className="mt-1 font-mono">Serial: {diagnostic.certificates[0]?.serial_suffix} · {diagnostic.certificates[0]?.status}</div>
                )}
              </div>
            )}
                </>
              )
            })()}
          </div>
        ))}
      </CardContent>
    </Card>
  )
}
