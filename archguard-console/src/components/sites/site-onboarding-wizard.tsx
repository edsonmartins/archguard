// W-C1 — AWS Console–style "New client" wizard (single happy path)

import { useMemo, useState } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import { useMutation } from '@tanstack/react-query'
import {
  ArrowLeft,
  ArrowRight,
  Building2,
  Cable,
  CheckCircle2,
  Circle,
  Loader2,
  Plus,
  Radar,
  Server,
  Sparkles,
  Trash2,
  XCircle,
} from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import {
  runClientOnboardingWizardFn,
  type WizardStepResult,
} from '@/server/onboarding-fn'
import { usePermissions } from '@/lib/hooks/use-permissions'

type StepId = 'identity' | 'network' | 'targets' | 'review' | 'done'

type ConnectorDraft = {
  id: string
  stack: 'openfortivpn' | 'openvpn'
  subnetsText: string
  materialize: boolean
  fortiHost: string
  fortiUser: string
  fortiPass: string
  config: string
}

type TargetDraft = {
  nome: string
  protocolo: string
  host: string
  port: number
  engine: 'warpgate' | 'guacamole'
  connector_id: string
  username: string
}

const STEPS: { id: StepId; label: string; icon: typeof Building2 }[] = [
  { id: 'identity', label: 'Identidade', icon: Building2 },
  { id: 'network', label: 'Rede', icon: Cable },
  { id: 'targets', label: 'Targets', icon: Server },
  { id: 'review', label: 'Revisar', icon: Radar },
  { id: 'done', label: 'Concluído', icon: CheckCircle2 },
]

function slugify(s: string) {
  return s
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .slice(0, 48)
}

export function SiteOnboardingWizard() {
  const navigate = useNavigate()
  const { can } = usePermissions()
  const canWrite = can('sites:create') || can('sites:update') || can('system:admin')

  const [step, setStep] = useState<StepId>('identity')
  const [cliente, setCliente] = useState('')
  const [slug, setSlug] = useState('')
  const [slugTouched, setSlugTouched] = useState(false)
  const [tenant, setTenant] = useState('')
  const [ambiente, setAmbiente] = useState<
    'producao' | 'preprod' | 'lab' | 'staging'
  >('producao')
  const [rolesText, setRolesText] = useState('')
  const [notas, setNotas] = useState('')
  const [connectors, setConnectors] = useState<ConnectorDraft[]>([
    {
      id: '',
      stack: 'openfortivpn',
      subnetsText: '',
      materialize: false,
      fortiHost: '',
      fortiUser: '',
      fortiPass: '',
      config: '',
    },
  ])
  const [targets, setTargets] = useState<TargetDraft[]>([])
  const [applyGateways, setApplyGateways] = useState(true)
  const [probesText, setProbesText] = useState('')
  const [resultSteps, setResultSteps] = useState<WizardStepResult[]>([])
  const [resultSlug, setResultSlug] = useState('')
  const [resultOk, setResultOk] = useState(false)

  const stepIndex = STEPS.findIndex((s) => s.id === step)

  const roles = useMemo(
    () =>
      rolesText
        .split(/[,\n]/)
        .map((r) => r.trim())
        .filter(Boolean),
    [rolesText],
  )

  const onClienteChange = (v: string) => {
    setCliente(v)
    if (!slugTouched) {
      const s = slugify(v)
      setSlug(s)
      setTenant(s ? `tenant_${s}` : '')
      if (connectors[0] && !connectors[0].id) {
        setConnectors((cs) =>
          cs.map((c, i) =>
            i === 0 ? { ...c, id: s ? `connector-${s}` : '' } : c,
          ),
        )
      }
      if (!rolesText) {
        setRolesText(s ? `tenant-${s.replace(/_/g, '-')}` : '')
      }
    }
  }

  const run = useMutation({
    mutationFn: () => {
      const probes = probesText
        .split(/[,\n]/)
        .map((x) => x.trim())
        .filter(Boolean)
        .map((pair) => {
          const [host, portStr] = pair.split(':')
          return { host: host!, port: Number(portStr) || 22 }
        })
        .filter((p) => p.host)

      return runClientOnboardingWizardFn({
        data: {
          cliente,
          slug,
          tenant_group: tenant || undefined,
          ambiente,
          warpgate_roles: roles,
          notas,
          apply_gateways: applyGateways,
          probes,
          connectors: connectors
            .filter((c) => c.id.trim())
            .map((c) => ({
              id: c.id.trim(),
              stack: c.stack,
              subnets: c.subnetsText
                .split(/[,\n]/)
                .map((s) => s.trim())
                .filter(Boolean),
              materialize: c.materialize,
              config: c.config.trim() || undefined,
              forti:
                c.materialize &&
                c.stack === 'openfortivpn' &&
                c.fortiHost &&
                c.fortiUser &&
                c.fortiPass &&
                !c.config.trim()
                  ? {
                      host: c.fortiHost,
                      username: c.fortiUser,
                      password: c.fortiPass,
                    }
                  : undefined,
            })),
          targets: targets
            .filter((t) => t.nome && t.host)
            .map((t) => ({
              nome: t.nome,
              engine: t.engine,
              protocolo: t.protocolo || 'ssh',
              host: t.host,
              port: t.port || 22,
              roles: roles.length ? roles : [],
              username: t.username || undefined,
              connector_id: t.connector_id || undefined,
            })),
        },
      })
    },
    onSuccess: (res) => {
      setResultSteps(res.steps)
      setResultSlug(res.site.slug)
      setResultOk(res.ok)
      setStep('done')
      if (res.ok) toast.success(`Cliente ${res.site.cliente} provisionado`)
      else toast.message('Wizard terminou com avisos — veja os passos')
    },
    onError: (e) => toast.error((e as Error).message),
  })

  if (!canWrite) {
    return (
      <div className="p-6">
        Sem permissão para criar clientes.{' '}
        <Link to="/sites" className="underline text-primary">
          Voltar
        </Link>
      </div>
    )
  }

  const nextFromIdentity = () => {
    if (!cliente.trim() || !slug.trim()) {
      toast.error('Informe nome e slug')
      return
    }
    setStep('network')
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6 p-6">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/sites">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <Sparkles className="h-6 w-6 text-primary" />
            Novo cliente
          </h1>
          <p className="text-sm text-muted-foreground">
            Fluxo único (console tipo AWS): identidade → rede → targets →
            materializar.
          </p>
        </div>
      </div>

      {/* Stepper */}
      <div className="flex flex-wrap gap-2">
        {STEPS.filter((s) => s.id !== 'done' || step === 'done').map((s, i) => {
          const active = s.id === step
          const done =
            STEPS.findIndex((x) => x.id === step) > i || step === 'done'
          const Icon = s.icon
          return (
            <Badge
              key={s.id}
              variant={active ? 'default' : done ? 'secondary' : 'outline'}
              className="gap-1 py-1.5 px-2.5"
            >
              <Icon className="h-3.5 w-3.5" />
              {i + 1}. {s.label}
            </Badge>
          )
        })}
      </div>

      {step === 'identity' && (
        <Card>
          <CardHeader>
            <CardTitle>1. Identidade do cliente</CardTitle>
            <CardDescription>
              Cria o site no inventário e o grupo tenant no Kanidm.
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1 sm:col-span-2">
              <Label>Nome comercial</Label>
              <Input
                value={cliente}
                onChange={(e) => onClienteChange(e.target.value)}
                placeholder="Rio Quality"
              />
            </div>
            <div className="space-y-1">
              <Label>Slug</Label>
              <Input
                className="font-mono"
                value={slug}
                onChange={(e) => {
                  setSlugTouched(true)
                  setSlug(slugify(e.target.value) || e.target.value)
                }}
                placeholder="rio_quality"
              />
            </div>
            <div className="space-y-1">
              <Label>Grupo tenant Kanidm</Label>
              <Input
                className="font-mono"
                value={tenant}
                onChange={(e) => setTenant(e.target.value)}
                placeholder="tenant_rio_quality"
              />
            </div>
            <div className="space-y-1">
              <Label>Ambiente</Label>
              <Select
                value={ambiente}
                onValueChange={(v) =>
                  setAmbiente(v as typeof ambiente)
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="producao">produção</SelectItem>
                  <SelectItem value="staging">staging</SelectItem>
                  <SelectItem value="preprod">preprod</SelectItem>
                  <SelectItem value="lab">lab</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1">
              <Label>Roles Warpgate</Label>
              <Input
                className="font-mono"
                value={rolesText}
                onChange={(e) => setRolesText(e.target.value)}
                placeholder="tenant-rio-quality"
              />
            </div>
            <div className="space-y-1 sm:col-span-2">
              <Label>Notas</Label>
              <Textarea
                value={notas}
                onChange={(e) => setNotas(e.target.value)}
                rows={2}
              />
            </div>
            <div className="sm:col-span-2 flex justify-end">
              <Button onClick={nextFromIdentity}>
                Continuar <ArrowRight className="h-4 w-4 ml-1" />
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {step === 'network' && (
        <Card>
          <CardHeader>
            <CardTitle>2. Rede (connectors)</CardTitle>
            <CardDescription>
              Paths VPN institucionais. Marque “Materializar” para gravar no
              host via agent (senha one-shot, não fica no inventário).
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {connectors.map((c, idx) => (
              <div
                key={idx}
                className="rounded-lg border p-4 space-y-3"
              >
                <div className="flex justify-between items-center">
                  <span className="font-medium text-sm">
                    Connector {idx + 1}
                  </span>
                  {connectors.length > 1 && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      onClick={() =>
                        setConnectors((cs) => cs.filter((_, i) => i !== idx))
                      }
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  )}
                </div>
                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="space-y-1">
                    <Label>ID</Label>
                    <Input
                      className="font-mono"
                      value={c.id}
                      onChange={(e) =>
                        setConnectors((cs) =>
                          cs.map((x, i) =>
                            i === idx ? { ...x, id: e.target.value } : x,
                          ),
                        )
                      }
                      placeholder="rio-forti"
                    />
                  </div>
                  <div className="space-y-1">
                    <Label>Stack</Label>
                    <Select
                      value={c.stack}
                      onValueChange={(v) =>
                        setConnectors((cs) =>
                          cs.map((x, i) =>
                            i === idx
                              ? {
                                  ...x,
                                  stack: v as 'openfortivpn' | 'openvpn',
                                }
                              : x,
                          ),
                        )
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="openfortivpn">
                          Forti (openfortivpn)
                        </SelectItem>
                        <SelectItem value="openvpn">OpenVPN</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1 sm:col-span-2">
                    <Label>Subnets (CIDR, vírgula)</Label>
                    <Input
                      value={c.subnetsText}
                      onChange={(e) =>
                        setConnectors((cs) =>
                          cs.map((x, i) =>
                            i === idx
                              ? { ...x, subnetsText: e.target.value }
                              : x,
                          ),
                        )
                      }
                      placeholder="10.1.0.0/16"
                    />
                  </div>
                  <div className="flex items-center gap-2 sm:col-span-2">
                    <Switch
                      checked={c.materialize}
                      onCheckedChange={(v) =>
                        setConnectors((cs) =>
                          cs.map((x, i) =>
                            i === idx ? { ...x, materialize: v } : x,
                          ),
                        )
                      }
                    />
                    <Label>Materializar agora no host (agent)</Label>
                  </div>
                  {c.materialize && c.stack === 'openfortivpn' && (
                    <>
                      <div className="space-y-1">
                        <Label>Host Forti</Label>
                        <Input
                          value={c.fortiHost}
                          onChange={(e) =>
                            setConnectors((cs) =>
                              cs.map((x, i) =>
                                i === idx
                                  ? { ...x, fortiHost: e.target.value }
                                  : x,
                              ),
                            )
                          }
                        />
                      </div>
                      <div className="space-y-1">
                        <Label>Usuário serviço</Label>
                        <Input
                          value={c.fortiUser}
                          onChange={(e) =>
                            setConnectors((cs) =>
                              cs.map((x, i) =>
                                i === idx
                                  ? { ...x, fortiUser: e.target.value }
                                  : x,
                              ),
                            )
                          }
                        />
                      </div>
                      <div className="space-y-1 sm:col-span-2">
                        <Label>Senha (one-shot)</Label>
                        <Input
                          type="password"
                          autoComplete="new-password"
                          value={c.fortiPass}
                          onChange={(e) =>
                            setConnectors((cs) =>
                              cs.map((x, i) =>
                                i === idx
                                  ? { ...x, fortiPass: e.target.value }
                                  : x,
                              ),
                            )
                          }
                        />
                      </div>
                    </>
                  )}
                  {c.materialize && (
                    <div className="space-y-1 sm:col-span-2">
                      <Label>
                        {c.stack === 'openvpn'
                          ? 'Conteúdo .ovpn'
                          : 'Ou conf completa (opcional)'}
                      </Label>
                      <Textarea
                        className="font-mono text-xs min-h-[100px]"
                        value={c.config}
                        onChange={(e) =>
                          setConnectors((cs) =>
                            cs.map((x, i) =>
                              i === idx
                                ? { ...x, config: e.target.value }
                                : x,
                            ),
                          )
                        }
                      />
                    </div>
                  )}
                </div>
              </div>
            ))}
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() =>
                setConnectors((cs) => [
                  ...cs,
                  {
                    id: slug ? `connector-${slug}-${cs.length + 1}` : '',
                    stack: 'openvpn',
                    subnetsText: '',
                    materialize: false,
                    fortiHost: '',
                    fortiUser: '',
                    fortiPass: '',
                    config: '',
                  },
                ])
              }
            >
              <Plus className="h-4 w-4 mr-1" /> Connector
            </Button>
            <Separator />
            <div className="flex justify-between">
              <Button variant="outline" onClick={() => setStep('identity')}>
                Voltar
              </Button>
              <Button onClick={() => setStep('targets')}>
                Continuar <ArrowRight className="h-4 w-4 ml-1" />
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {step === 'targets' && (
        <Card>
          <CardHeader>
            <CardTitle>3. Targets</CardTitle>
            <CardDescription>
              Hosts internos alcançáveis via connector. Apply no Warpgate/Guac
              no passo final.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {targets.length === 0 && (
              <p className="text-sm text-muted-foreground">
                Nenhum target — pode pular e cadastrar depois.
              </p>
            )}
            {targets.map((t, idx) => (
              <div
                key={idx}
                className="grid gap-2 sm:grid-cols-6 rounded border p-3"
              >
                <Input
                  className="sm:col-span-2"
                  placeholder="nome"
                  value={t.nome}
                  onChange={(e) =>
                    setTargets((ts) =>
                      ts.map((x, i) =>
                        i === idx ? { ...x, nome: e.target.value } : x,
                      ),
                    )
                  }
                />
                <Input
                  placeholder="ssh|postgres|https"
                  value={t.protocolo}
                  onChange={(e) =>
                    setTargets((ts) =>
                      ts.map((x, i) =>
                        i === idx ? { ...x, protocolo: e.target.value } : x,
                      ),
                    )
                  }
                />
                <Input
                  placeholder="host"
                  value={t.host}
                  onChange={(e) =>
                    setTargets((ts) =>
                      ts.map((x, i) =>
                        i === idx ? { ...x, host: e.target.value } : x,
                      ),
                    )
                  }
                />
                <Input
                  type="number"
                  placeholder="port"
                  value={t.port}
                  onChange={(e) =>
                    setTargets((ts) =>
                      ts.map((x, i) =>
                        i === idx
                          ? { ...x, port: Number(e.target.value) || 22 }
                          : x,
                      ),
                    )
                  }
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  onClick={() =>
                    setTargets((ts) => ts.filter((_, i) => i !== idx))
                  }
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            ))}
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() =>
                setTargets((ts) => [
                  ...ts,
                  {
                    nome: '',
                    protocolo: 'ssh',
                    host: '',
                    port: 22,
                    engine: 'warpgate',
                    connector_id: connectors[0]?.id || '',
                    username: '',
                  },
                ])
              }
            >
              <Plus className="h-4 w-4 mr-1" /> Target
            </Button>
            <div className="space-y-1">
              <Label>Probes TCP (opcional, host:port por linha)</Label>
              <Textarea
                className="font-mono text-xs"
                value={probesText}
                onChange={(e) => setProbesText(e.target.value)}
                placeholder="10.1.2.3:22"
                rows={2}
              />
            </div>
            <div className="flex items-center gap-2">
              <Switch
                checked={applyGateways}
                onCheckedChange={setApplyGateways}
              />
              <Label>Aplicar targets nos gateways ao finalizar</Label>
            </div>
            <div className="flex justify-between">
              <Button variant="outline" onClick={() => setStep('network')}>
                Voltar
              </Button>
              <Button onClick={() => setStep('review')}>
                Revisar <ArrowRight className="h-4 w-4 ml-1" />
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {step === 'review' && (
        <Card>
          <CardHeader>
            <CardTitle>4. Revisar e provisionar</CardTitle>
            <CardDescription>
              Uma ação cria site, tenant Kanidm, connectors (se marcados) e
              aplica gateways.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 text-sm">
            <dl className="grid gap-2 sm:grid-cols-2">
              <div>
                <dt className="text-muted-foreground">Cliente</dt>
                <dd className="font-medium">{cliente}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Slug / tenant</dt>
                <dd className="font-mono">
                  {slug} / {tenant || `tenant_${slug}`}
                </dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Connectors</dt>
                <dd>
                  {connectors.filter((c) => c.id).length} (
                  {connectors.filter((c) => c.materialize).length} materializar)
                </dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Targets</dt>
                <dd>
                  {targets.filter((t) => t.nome && t.host).length} · apply=
                  {applyGateways ? 'sim' : 'não'}
                </dd>
              </div>
            </dl>
            <div className="flex justify-between">
              <Button variant="outline" onClick={() => setStep('targets')}>
                Voltar
              </Button>
              <Button
                disabled={run.isPending}
                onClick={() => run.mutate()}
              >
                {run.isPending ? (
                  <>
                    <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                    Provisionando…
                  </>
                ) : (
                  <>
                    <Sparkles className="h-4 w-4 mr-2" />
                    Provisionar cliente
                  </>
                )}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {step === 'done' && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              {resultOk ? (
                <CheckCircle2 className="h-6 w-6 text-emerald-600" />
              ) : (
                <XCircle className="h-6 w-6 text-amber-600" />
              )}
              {resultOk ? 'Cliente provisionado' : 'Concluído com avisos'}
            </CardTitle>
            <CardDescription>
              Site{' '}
              <span className="font-mono font-medium">{resultSlug}</span>
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <ul className="space-y-2">
              {resultSteps.map((s, i) => (
                <li
                  key={`${s.step}-${i}`}
                  className="flex items-start gap-2 text-sm"
                >
                  {s.ok ? (
                    <CheckCircle2 className="h-4 w-4 text-emerald-600 shrink-0 mt-0.5" />
                  ) : (
                    <Circle className="h-4 w-4 text-amber-600 shrink-0 mt-0.5" />
                  )}
                  <span>
                    <span className="font-mono text-xs">{s.step}</span>
                    {s.detail ? (
                      <span className="text-muted-foreground">
                        {' '}
                        — {s.detail}
                      </span>
                    ) : null}
                  </span>
                </li>
              ))}
            </ul>
            <div className="flex flex-wrap gap-2">
              <Button asChild>
                <Link to="/sites/$slug" params={{ slug: resultSlug }}>
                  Abrir site
                </Link>
              </Button>
              <Button variant="outline" asChild>
                <Link to="/identities">Identidades / operadores</Link>
              </Button>
              <Button variant="outline" asChild>
                <Link to="/gateways">Gateways</Link>
              </Button>
              <Button
                variant="ghost"
                onClick={() => void navigate({ to: '/sites' })}
              >
                Lista de clientes
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {step !== 'done' && (
        <p className="text-xs text-muted-foreground text-center">
          Passo {stepIndex + 1} de {STEPS.length - 1} · control plane console —
          sem SSH no happy path
        </p>
      )}
    </div>
  )
}
