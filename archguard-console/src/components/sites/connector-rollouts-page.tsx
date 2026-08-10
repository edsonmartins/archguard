import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Rocket, Play, RotateCcw, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import { Skeleton } from '@/components/ui/skeleton'
import { PageHeader } from '@/components/shared/page-header'
import { usePermissions } from '@/lib/hooks/use-permissions'
import { useSites } from '@/lib/hooks/use-sites'
import {
  advanceConnectorUpgradeRolloutFn,
  createConnectorUpgradeRolloutFn,
  getConnectorUpgradePlansFn,
  listConnectorUpgradeRolloutsFn,
  rollbackConnectorUpgradeRolloutFn,
} from '@/server/connector-fn'

type ApprovedPlan = { slug: string; plan_id: string; version: string; artifact_url: string; sha256: string }

export function ConnectorRolloutsPage() {
  const { can } = usePermissions()
  const canWrite = can('sites:update') || can('system:admin')
  const qc = useQueryClient()
  const { data: sites } = useSites()
  const [selected, setSelected] = useState<Record<string, boolean>>({})
  const [batchSize, setBatchSize] = useState('1')

  const plans = useQuery({
    queryKey: ['approved-connector-plans', sites?.map((site) => site.slug).join(',')],
    enabled: Boolean(sites?.length),
    queryFn: async () => {
      const result = await Promise.all((sites || []).map(async (site) => {
        const response = await getConnectorUpgradePlansFn({ data: { slug: site.slug } })
        return response.plans.filter((plan) => plan.status === 'approved').map((plan) => ({
          slug: site.slug,
          plan_id: plan.id,
          version: plan.version,
          artifact_url: plan.artifact_url,
          sha256: plan.sha256,
        }))
      }))
      return result.flat()
    },
    staleTime: 10_000,
  })
  const rollouts = useQuery({
    queryKey: ['connector-upgrade-rollouts'],
    queryFn: () => listConnectorUpgradeRolloutsFn(),
    staleTime: 10_000,
  })
  const create = useMutation({
    mutationFn: () => {
      const targets = (plans.data || []).filter((plan) => selected[plan.plan_id]).map(({ slug, plan_id }) => ({ slug, plan_id }))
      if (!targets.length) throw new Error('Selecione ao menos um plano aprovado')
      return createConnectorUpgradeRolloutFn({ data: { targets, batch_size: Math.max(1, Math.min(10, Number(batchSize) || 1)) } })
    },
    onSuccess: (result) => {
      toast.success(`Onda ${result.rollout_id} criada com ${result.target_count} alvo(s)`)
      setSelected({})
      void qc.invalidateQueries({ queryKey: ['connector-upgrade-rollouts'] })
    },
    onError: (error) => toast.error((error as Error).message),
  })
  const advance = useMutation({
    mutationFn: (rollout_id: string) => advanceConnectorUpgradeRolloutFn({ data: { rollout_id } }),
    onSuccess: (result) => {
      toast.success(result.ok ? `Onda: ${result.status}` : `Onda interrompida: ${result.error}`)
      void qc.invalidateQueries({ queryKey: ['connector-upgrade-rollouts'] })
      void qc.invalidateQueries({ queryKey: ['approved-connector-plans'] })
    },
    onError: (error) => toast.error((error as Error).message),
  })
  const rollback = useMutation({
    mutationFn: (rollout_id: string) => rollbackConnectorUpgradeRolloutFn({ data: { rollout_id } }),
    onSuccess: () => {
      toast.success('Rollback da onda concluído')
      void qc.invalidateQueries({ queryKey: ['connector-upgrade-rollouts'] })
    },
    onError: (error) => toast.error((error as Error).message),
  })
  const selectedPlans = useMemo(() => (plans.data || []).filter((plan) => selected[plan.plan_id]), [plans.data, selected])

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title={<span className="inline-flex items-center gap-2"><Rocket className="h-7 w-7 text-primary" /> Rollouts de connectors</span>}
        description="Atualizações graduais por lote, com parada no primeiro erro e rollback dos alvos aplicados."
        actions={<Button variant="outline" size="sm" onClick={() => { void plans.refetch(); void rollouts.refetch() }}><RefreshCw className="h-4 w-4 mr-1" /> Atualizar</Button>}
      />

      {canWrite && <Card>
        <CardHeader>
          <CardTitle className="text-base">Criar nova onda</CardTitle>
          <CardDescription>Selecione planos aprovados com o mesmo artefato e defina quantos hosts avançam por vez.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-end gap-3">
            <div className="space-y-1"><Label htmlFor="batch-size">Tamanho do lote</Label><Input id="batch-size" className="w-24" type="number" min="1" max="10" value={batchSize} onChange={(event) => setBatchSize(event.target.value)} /></div>
            <Button disabled={create.isPending || selectedPlans.length === 0} onClick={() => create.mutate()}>Criar onda ({selectedPlans.length})</Button>
          </div>
          {plans.isLoading && <Skeleton className="h-16 w-full" />}
          {plans.isError && <p className="text-sm text-destructive">{(plans.error as Error).message}</p>}
          {!plans.isLoading && !plans.isError && (plans.data || []).length === 0 && <p className="text-sm text-muted-foreground">Nenhum plano aprovado disponível.</p>}
          <div className="space-y-2">
            {(plans.data || []).map((plan) => <label key={plan.plan_id} className="flex items-start gap-3 rounded border p-3 cursor-pointer">
              <Checkbox checked={Boolean(selected[plan.plan_id])} onCheckedChange={(checked) => setSelected((current) => ({ ...current, [plan.plan_id]: checked === true }))} />
              <span className="min-w-0 text-sm"><span className="font-medium">{plan.slug} · {plan.version}</span><span className="block text-xs text-muted-foreground break-all">SHA: {plan.sha256}</span></span>
            </label>)}
          </div>
        </CardContent>
      </Card>}

      <Card>
        <CardHeader><CardTitle className="text-base">Ondas registradas</CardTitle><CardDescription>Avanço explícito por lote; aplicação reinicia o agent de cada host.</CardDescription></CardHeader>
        <CardContent className="space-y-3">
          {rollouts.isLoading && <Skeleton className="h-20 w-full" />}
          {rollouts.isError && <p className="text-sm text-destructive">{(rollouts.error as Error).message}</p>}
          {!rollouts.isLoading && !rollouts.isError && (rollouts.data?.rollouts || []).length === 0 && <p className="text-sm text-muted-foreground">Nenhuma onda criada.</p>}
          {(rollouts.data?.rollouts || []).map((rollout) => {
            const targets = (rollout.targets || []) as Array<{ site_slug: string; status: string; error?: string | null }>
            const applied = targets.filter((target) => target.status === 'applied').length
            return <div key={String(rollout.id)} className="rounded border p-3 space-y-2">
              <div className="flex flex-wrap items-center justify-between gap-2"><span className="font-medium">{String(rollout.version)} · <span className="font-mono text-xs">{String(rollout.id).slice(0, 8)}</span></span><Badge variant={rollout.status === 'failed' ? 'destructive' : rollout.status === 'completed' ? 'default' : 'outline'}>{String(rollout.status)}</Badge></div>
              <div className="text-xs text-muted-foreground">{applied}/{targets.length} aplicados · lote {String(rollout.batch_size)} · {new Date(String(rollout.created_at)).toLocaleString()}</div>
              <div className="flex flex-wrap gap-2">{targets.map((target) => <Badge key={target.site_slug} variant={target.status === 'failed' ? 'destructive' : target.status === 'applied' ? 'default' : 'secondary'}>{target.site_slug}: {target.status}</Badge>)}</div>
              {targets.some((target) => target.error) && <p className="text-xs text-destructive">{targets.find((target) => target.error)?.error}</p>}
              {canWrite && <div className="flex gap-2"><Button size="sm" disabled={advance.isPending || !['planned', 'running'].includes(String(rollout.status))} onClick={() => advance.mutate(String(rollout.id))}><Play className="h-3.5 w-3.5 mr-1" /> Avançar lote</Button><Button size="sm" variant="outline" disabled={rollback.isPending || !['completed', 'failed', 'running'].includes(String(rollout.status)) || applied === 0} onClick={() => rollback.mutate(String(rollout.id))}><RotateCcw className="h-3.5 w-3.5 mr-1" /> Rollback aplicados</Button></div>}
            </div>
          })}
        </CardContent>
      </Card>
    </div>
  )
}
