// Formulário visual de cliente/site (criar / editar)

import { useEffect, useState } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, Plus, Trash2, Save } from 'lucide-react'
import { toast } from 'sonner'
import { enumLabel } from '@/lib/i18n/labels'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { useSite, useUpsertSite } from '@/lib/hooks/use-sites'
import { usePermissions } from '@/lib/hooks/use-permissions'
import {
  SITE_AMBIENTES,
  SITE_STACKS,
  SITE_TIPOS,
  type Site,
  type SiteConnector,
  type SiteInput,
  type SiteTarget,
} from '@/lib/api/types/site'
import { Skeleton } from '@/components/ui/skeleton'

const emptyTarget = (): SiteTarget => ({
  nome: '',
  engine: 'warpgate',
  protocolo: 'ssh',
  host: '',
  port: 22,
  roles: [],
  secret_ref: '',
  connector_id: '',
})

const emptyConnector = (slug = ''): SiteConnector => ({
  id: slug ? `connector-${slug}` : '',
  stack: 'a_confirmar',
  tipo: 'connector_peer',
  subnets: [],
  meta: {},
  notas: '',
})

function toInput(site: Site): SiteInput {
  const { updated_at: _a, updated_by: _b, ...rest } = site
  return {
    ...rest,
    connectors: rest.connectors?.length
      ? rest.connectors
      : [
          {
            id: rest.connector_id || `connector-${rest.slug}`,
            stack: rest.stack,
            tipo: rest.tipo,
            subnets: rest.subnets,
            meta: rest.stack_meta,
          },
        ],
  }
}

function blank(slugHint = ''): SiteInput {
  return {
    slug: slugHint,
    cliente: '',
    tenant_group: slugHint ? `tenant_${slugHint}` : '',
    ambiente: 'producao',
    tipo: 'a_confirmar',
    stack: 'a_confirmar',
    connector_id: '',
    subnets: [],
    stack_meta: {},
    connectors: [emptyConnector(slugHint)],
    targets: [],
    warpgate_roles: [],
    notas: '',
    inventariado: false,
    connector_deployed: false,
    smoke_operador: false,
  }
}

export function SiteFormPage({
  mode,
  slug,
}: {
  mode: 'create' | 'edit'
  slug?: string
}) {
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { can } = usePermissions()
  const canWrite = can('sites:update') || can('sites:create')
  const { data: existing, isLoading } = useSite(slug || '')
  const upsert = useUpsertSite()
  const [form, setForm] = useState<SiteInput>(blank())
  const [subnetsText, setSubnetsText] = useState('')
  const [rolesText, setRolesText] = useState('')
  const [metaHost, setMetaHost] = useState('')
  const [metaProfile, setMetaProfile] = useState('')
  const [metaMfa, setMetaMfa] = useState(false)

  useEffect(() => {
    if (mode === 'edit' && existing) {
      setForm(toInput(existing))
      setSubnetsText((existing.subnets || []).join('\n'))
      setRolesText((existing.warpgate_roles || []).join(', '))
      setMetaHost(String(existing.stack_meta?.host || existing.stack_meta?.endpoint || ''))
      setMetaProfile(String(existing.stack_meta?.profile_ref || ''))
      setMetaMfa(!!existing.stack_meta?.mfa)
    }
  }, [mode, existing])

  if (mode === 'edit' && isLoading) {
    return (
      <div className="p-6 space-y-3">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-40 w-full" />
      </div>
    )
  }

  if (mode === 'edit' && !isLoading && !existing) {
    return (
      <div className="p-6">
        Site não encontrado.{' '}
        <Link to="/sites" className="text-primary underline">
          Voltar
        </Link>
      </div>
    )
  }

  const set = <K extends keyof SiteInput>(key: K, value: SiteInput[K]) =>
    setForm((f) => ({ ...f, [key]: value }))

  const onSlugChange = (v: string) => {
    const slug = v.toLowerCase().replace(/[^a-z0-9_-]/g, '_')
    setForm((f) => ({
      ...f,
      slug,
      tenant_group: f.tenant_group || (slug ? `tenant_${slug}` : ''),
      connector_id: f.connector_id || (slug ? `connector-${slug}` : ''),
    }))
  }

  const save = async () => {
    if (!canWrite) {
      toast.error(t('sites.noPermission'))
      return
    }
    const connectors = (form.connectors?.length
      ? form.connectors
      : [emptyConnector(form.slug)]
    ).map((c, i) =>
      i === 0
        ? {
            ...c,
            id: c.id || form.connector_id || `connector-${form.slug}`,
            stack: c.stack || form.stack,
            tipo: c.tipo || form.tipo,
            subnets: subnetsText
              .split(/[\n,]/)
              .map((s) => s.trim())
              .filter(Boolean),
            meta: {
              ...(c.meta || {}),
              ...(metaHost ? { host: metaHost, endpoint: metaHost } : {}),
              ...(metaProfile ? { profile_ref: metaProfile } : {}),
              mfa: metaMfa,
            },
          }
        : c,
    )
    const payload: SiteInput = {
      ...form,
      connectors,
      connector_id: connectors[0]?.id || form.connector_id,
      stack:
        connectors.length > 1
          ? 'mixed'
          : connectors[0]?.stack || form.stack,
      tipo: connectors[0]?.tipo || form.tipo,
      subnets: connectors[0]?.subnets || [],
      stack_meta: connectors[0]?.meta || form.stack_meta,
      warpgate_roles: rolesText
        .split(/[,\n]/)
        .map((s) => s.trim())
        .filter(Boolean),
      targets: form.targets.map((t) => ({
        ...t,
        secret_ref: t.secret_ref?.trim() || undefined,
        connector_id: t.connector_id?.trim() || undefined,
        roles:
          typeof t.roles === 'string'
            ? String(t.roles)
                .split(',')
                .map((r) => r.trim())
                .filter(Boolean)
            : t.roles,
      })),
    }
    try {
      const site = await upsert.mutateAsync(payload)
      toast.success(t('sites.saved', { name: site.cliente }))
      void navigate({ to: '/sites/$slug', params: { slug: site.slug } })
    } catch (e) {
      toast.error((e as Error).message)
    }
  }

  const updateTarget = (idx: number, patch: Partial<SiteTarget>) => {
    setForm((f) => {
      const targets = [...f.targets]
      targets[idx] = { ...targets[idx]!, ...patch }
      return { ...f, targets }
    })
  }

  return (
    <div className="space-y-6 p-6 max-w-4xl">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon" asChild>
          <Link to={mode === 'edit' && slug ? '/sites/$slug' : '/sites'} params={slug ? { slug } : undefined}>
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div>
          <h1 className="text-2xl font-bold">
            {mode === 'create'
              ? t('sites.createTitle')
              : t('sites.edit', { name: form.cliente || slug })}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t('sites.subtitle')}
          </p>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Identidade do cliente</CardTitle>
          <CardDescription>Ligado ao tenant Kanidm (`tenant_*`).</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2 sm:col-span-2">
            <Label>Nome comercial</Label>
            <Input
              value={form.cliente}
              onChange={(e) => set('cliente', e.target.value)}
              placeholder="Rio Quality"
              disabled={!canWrite}
            />
          </div>
          <div className="space-y-2">
            <Label>Slug</Label>
            <Input
              value={form.slug}
              onChange={(e) => onSlugChange(e.target.value)}
              placeholder="rio_quality"
              disabled={!canWrite || mode === 'edit'}
              className="font-mono"
            />
          </div>
          <div className="space-y-2">
            <Label>Grupo tenant Kanidm</Label>
            <Input
              value={form.tenant_group}
              onChange={(e) => set('tenant_group', e.target.value)}
              className="font-mono"
              disabled={!canWrite}
            />
          </div>
          <div className="space-y-2">
            <Label>{t('sites.fields.environment')}</Label>
            <Select
              value={form.ambiente}
              onValueChange={(v) => set('ambiente', v as SiteInput['ambiente'])}
              disabled={!canWrite}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SITE_AMBIENTES.map((a) => (
                  <SelectItem key={a} value={a}>
                    {enumLabel(t, 'ambiente', a)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>{t('sites.fields.warpgateRoles')}</Label>
            <Input
              value={rolesText}
              onChange={(e) => setRolesText(e.target.value)}
              placeholder="tenant-rio-quality"
              disabled={!canWrite}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('sites.connectivity')}</CardTitle>
          <CardDescription>{t('sites.connectivityHint')}</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <Label>{t('sites.fields.type')}</Label>
            <Select
              value={form.tipo}
              onValueChange={(v) => set('tipo', v as SiteInput['tipo'])}
              disabled={!canWrite}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SITE_TIPOS.map((tipo) => (
                  <SelectItem key={tipo} value={tipo}>
                    {enumLabel(t, 'tipo', tipo)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>{t('sites.fields.stack')}</Label>
            <Select
              value={form.stack}
              onValueChange={(v) => set('stack', v as SiteInput['stack'])}
              disabled={!canWrite}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SITE_STACKS.map((stack) => (
                  <SelectItem key={stack} value={stack}>
                    {enumLabel(t, 'stack', stack)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>{t('sites.fields.connectorId')}</Label>
            <Input
              value={form.connector_id}
              onChange={(e) => set('connector_id', e.target.value)}
              className="font-mono"
              disabled={!canWrite}
            />
          </div>
          <div className="space-y-2">
            <Label>{t('sites.fields.vpnHost')}</Label>
            <Input
              value={metaHost}
              onChange={(e) => setMetaHost(e.target.value)}
              placeholder="vpn.cliente.example:443"
              disabled={!canWrite}
            />
          </div>
          <div className="space-y-2">
            <Label>Profile ref OpenVPN (path no host, sem segredo)</Label>
            <Input
              value={metaProfile}
              onChange={(e) => setMetaProfile(e.target.value)}
              placeholder="/var/lib/archgate/connector/…/client.ovpn"
              disabled={!canWrite}
            />
          </div>
          <div className="flex items-center gap-2 pt-6">
            <Switch checked={metaMfa} onCheckedChange={setMetaMfa} disabled={!canWrite} />
            <Label>VPN exige MFA/OTP</Label>
          </div>
          <div className="space-y-2 sm:col-span-2">
            <Label>Subnets alvo (uma por linha)</Label>
            <Textarea
              value={subnetsText}
              onChange={(e) => setSubnetsText(e.target.value)}
              placeholder="10.50.0.0/24"
              rows={3}
              disabled={!canWrite}
            />
          </div>
          <div className="space-y-2 sm:col-span-2">
            <Label>Notas</Label>
            <Textarea
              value={form.notas}
              onChange={(e) => set('notas', e.target.value)}
              rows={3}
              disabled={!canWrite}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle>Connectors (multi-VPN)</CardTitle>
            <CardDescription>
              Rio Quality: um openfortivpn (LAN/Oracle) + um OpenVPN (AWS). O
              primeiro espelha os campos de conectividade acima.
            </CardDescription>
          </div>
          {canWrite && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() =>
                setForm((f) => ({
                  ...f,
                  connectors: [
                    ...(f.connectors?.length
                      ? f.connectors
                      : [emptyConnector(f.slug)]),
                    {
                      id: `connector-${f.slug || 'site'}-${(f.connectors?.length || 1) + 1}`,
                      stack: 'openvpn',
                      tipo: f.tipo || 'tunnel_agent',
                      subnets: [],
                      meta: {},
                    },
                  ],
                }))
              }
            >
              <Plus className="h-4 w-4 mr-1" /> Connector
            </Button>
          )}
        </CardHeader>
        <CardContent className="space-y-3">
          {(form.connectors?.length
            ? form.connectors
            : [emptyConnector(form.slug)]
          ).map((c, idx) => (
            <div
              key={idx}
              className="rounded-lg border p-3 grid gap-2 sm:grid-cols-6"
            >
              <Input
                className="sm:col-span-2 font-mono"
                placeholder="id"
                value={c.id}
                onChange={(e) => {
                  const id = e.target.value
                  setForm((f) => {
                    const list = [...(f.connectors || [])]
                    if (!list.length) list.push(emptyConnector(f.slug))
                    list[idx] = { ...list[idx]!, id }
                    return { ...f, connectors: list, connector_id: list[0]?.id || f.connector_id }
                  })
                }}
                disabled={!canWrite || idx === 0}
              />
              <Select
                value={c.stack}
                onValueChange={(v) => {
                  setForm((f) => {
                    const list = [...(f.connectors || [emptyConnector(f.slug)])]
                    list[idx] = {
                      ...list[idx]!,
                      stack: v as SiteConnector['stack'],
                    }
                    return {
                      ...f,
                      connectors: list,
                      stack:
                        list.length > 1
                          ? 'mixed'
                          : (list[0]?.stack as SiteInput['stack']),
                    }
                  })
                }}
                disabled={!canWrite}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SITE_STACKS.map((stack) => (
                    <SelectItem key={stack} value={stack}>
                      {enumLabel(t, 'stack', stack)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Input
                className="sm:col-span-2"
                placeholder="subnets (csv)"
                value={(c.subnets || []).join(', ')}
                onChange={(e) => {
                  const subnets = e.target.value
                    .split(/[,\n]/)
                    .map((s) => s.trim())
                    .filter(Boolean)
                  setForm((f) => {
                    const list = [...(f.connectors || [emptyConnector(f.slug)])]
                    list[idx] = { ...list[idx]!, subnets }
                    return { ...f, connectors: list }
                  })
                }}
                disabled={!canWrite || idx === 0}
              />
              {canWrite && idx > 0 && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  onClick={() =>
                    setForm((f) => ({
                      ...f,
                      connectors: (f.connectors || []).filter((_, i) => i !== idx),
                    }))
                  }
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              )}
            </div>
          ))}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle>Targets</CardTitle>
            <CardDescription>
              Metadado inventário. secret_ref = path OpenBao (sem senha no SoT).
              Apply materializa no Warpgate/Guac.
            </CardDescription>
          </div>
          {canWrite && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() =>
                setForm((f) => ({
                  ...f,
                  targets: [...f.targets, emptyTarget()],
                }))
              }
            >
              <Plus className="h-4 w-4 mr-1" /> Target
            </Button>
          )}
        </CardHeader>
        <CardContent className="space-y-4">
          {form.targets.length === 0 && (
            <p className="text-sm text-muted-foreground">Nenhum target cadastrado.</p>
          )}
          {form.targets.map((tgt, idx) => (
            <div key={idx} className="rounded-lg border p-3 grid gap-2 sm:grid-cols-6">
              <Input
                className="sm:col-span-2"
                placeholder="nome"
                value={tgt.nome}
                onChange={(e) => updateTarget(idx, { nome: e.target.value })}
                disabled={!canWrite}
              />
              <Select
                value={tgt.engine}
                onValueChange={(v) =>
                  updateTarget(idx, { engine: v as 'warpgate' | 'guacamole' })
                }
                disabled={!canWrite}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('sites.fields.engine')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="warpgate">
                    {enumLabel(t, 'engine', 'warpgate')}
                  </SelectItem>
                  <SelectItem value="guacamole">
                    {enumLabel(t, 'engine', 'guacamole')}
                  </SelectItem>
                </SelectContent>
              </Select>
              <Input
                placeholder="protocolo"
                value={tgt.protocolo}
                onChange={(e) => updateTarget(idx, { protocolo: e.target.value })}
                disabled={!canWrite}
              />
              <Input
                placeholder="host"
                value={tgt.host}
                onChange={(e) => updateTarget(idx, { host: e.target.value })}
                disabled={!canWrite}
              />
              <Input
                type="number"
                placeholder="port"
                value={tgt.port}
                onChange={(e) =>
                  updateTarget(idx, { port: Number(e.target.value) || 0 })
                }
                disabled={!canWrite}
              />
              <Input
                className="sm:col-span-2 font-mono text-xs"
                placeholder="secret_ref OpenBao (ex: secret/data/archgate/targets/rio-ssh)"
                value={tgt.secret_ref || ''}
                onChange={(e) =>
                  updateTarget(idx, { secret_ref: e.target.value })
                }
                disabled={!canWrite}
              />
              <Input
                className="sm:col-span-2 font-mono text-xs"
                placeholder="connector_id (opcional)"
                value={tgt.connector_id || ''}
                onChange={(e) =>
                  updateTarget(idx, { connector_id: e.target.value })
                }
                disabled={!canWrite}
              />
              <div className="flex gap-2 sm:col-span-6 justify-end">
                {canWrite && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    onClick={() =>
                      setForm((f) => ({
                        ...f,
                        targets: f.targets.filter((_, i) => i !== idx),
                      }))
                    }
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                )}
              </div>
              <Input
                className="sm:col-span-6"
                placeholder="roles (vírgula)"
                value={(tgt.roles || []).join(', ')}
                onChange={(e) =>
                  updateTarget(idx, {
                    roles: e.target.value
                      .split(',')
                      .map((r) => r.trim())
                      .filter(Boolean),
                  })
                }
                disabled={!canWrite}
              />
            </div>
          ))}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Estado onboarding</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-6">
          {(
            [
              ['inventariado', 'Inventariado'],
              ['connector_deployed', t('sites.connectorProd')],
              ['smoke_operador', 'Smoke operador OK'],
            ] as const
          ).map(([key, label]) => (
            <div key={key} className="flex items-center gap-2">
              <Switch
                checked={!!form[key]}
                onCheckedChange={(v) => set(key, v)}
                disabled={!canWrite}
              />
              <Label>{label}</Label>
            </div>
          ))}
        </CardContent>
      </Card>

      <Separator />

      <div className="flex justify-end gap-2">
        <Button variant="outline" asChild>
          <Link to="/sites">Cancelar</Link>
        </Button>
        {canWrite && (
          <Button onClick={() => void save()} disabled={upsert.isPending}>
            <Save className="h-4 w-4 mr-2" />
            Salvar
          </Button>
        )}
      </div>
    </div>
  )
}
