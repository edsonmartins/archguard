// src/components/oauth2/oauth2-create-wizard.tsx

import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useForm } from '@tanstack/react-form'
import {
  ArrowLeft,
  ArrowRight,
  Check,
  Globe,
  Shield,
  Settings,
  ClipboardCheck,
  Plus,
  X,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { useCreateOAuth2Client, useSetScopeMap } from '@/lib/hooks/use-oauth2'
import { useGroups } from '@/lib/hooks/use-groups'

const STEPS = [
  { id: 'type', label: 'Tipo', icon: Shield },
  { id: 'config', label: 'Configuração', icon: Settings },
  { id: 'scopes', label: 'Scope Maps', icon: Globe },
  { id: 'review', label: 'Revisão', icon: ClipboardCheck },
] as const

interface ScopeMapEntry {
  groupId: string
  groupName: string
  scopes: string[]
}

export function OAuth2CreateWizard() {
  const navigate = useNavigate()
  const createClient = useCreateOAuth2Client()
  const setScopeMap = useSetScopeMap()
  const { data: groups } = useGroups()

  const [step, setStep] = useState(0)
  const [redirectInput, setRedirectInput] = useState('')
  const [scopeMaps, setScopeMaps] = useState<ScopeMapEntry[]>([])
  const [scopeGroupId, setScopeGroupId] = useState('')
  const [scopeInput, setScopeInput] = useState('')

  const form = useForm({
    defaultValues: {
      type: 'basic' as 'basic' | 'public',
      name: '',
      displayname: '',
      origin_landing: '',
      redirect_urls: [] as string[],
    },
    onSubmit: async ({ value }) => {
      createClient.mutate(
        {
          name: value.name,
          displayname: value.displayname,
          origin_landing: value.origin_landing,
          type: value.type,
        },
        {
          onSuccess: () => {
            // Apply scope maps if any
            if (scopeMaps.length > 0) {
              Promise.all(
                scopeMaps.map((sm) =>
                  setScopeMap.mutateAsync({
                    clientId: value.name,
                    groupId: sm.groupId,
                    scopes: sm.scopes,
                  }),
                ),
              ).then(() => navigate({ to: '/oauth2' }))
            } else {
              navigate({ to: '/oauth2' })
            }
          },
        },
      )
    },
  })

  const addRedirectUrl = () => {
    const url = redirectInput.trim()
    if (url) {
      try {
        new URL(url)
        const current = form.getFieldValue('redirect_urls')
        if (!current.includes(url)) {
          form.setFieldValue('redirect_urls', [...current, url])
        }
        setRedirectInput('')
      } catch {
        // invalid URL
      }
    }
  }

  const addScopeMap = () => {
    if (!scopeGroupId || !scopeInput.trim()) return
    const group = groups?.find((g) => g.id === scopeGroupId)
    if (!group) return

    const scopes = scopeInput.split(',').map((s) => s.trim()).filter(Boolean)
    setScopeMaps((prev) => [
      ...prev.filter((sm) => sm.groupId !== scopeGroupId),
      { groupId: scopeGroupId, groupName: group.name, scopes },
    ])
    setScopeGroupId('')
    setScopeInput('')
  }

  const canAdvance = () => {
    if (step === 1) {
      const v = form.state.values
      return v.name.length >= 2 && v.displayname.length >= 1 && v.origin_landing.length > 0
    }
    return true
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Novo Client OAuth2</h1>
        <p className="text-muted-foreground">
          Configure um novo cliente para integração via SSO
        </p>
      </div>

      {/* Step indicator */}
      <div className="flex items-center gap-2">
        {STEPS.map((s, i) => {
          const Icon = s.icon
          const isActive = i === step
          const isDone = i < step
          return (
            <div key={s.id} className="flex items-center gap-2">
              {i > 0 && <Separator className="w-6" />}
              <div
                className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm ${
                  isActive
                    ? 'bg-primary text-primary-foreground'
                    : isDone
                      ? 'bg-primary/10 text-primary'
                      : 'text-muted-foreground'
                }`}
              >
                {isDone ? <Check className="h-4 w-4" /> : <Icon className="h-4 w-4" />}
                {s.label}
              </div>
            </div>
          )
        })}
      </div>

      <Card>
        <CardContent className="pt-6">
          {step === 0 && (
            <div className="space-y-4">
              <p className="text-sm text-muted-foreground">
                Escolha o tipo de cliente OAuth2
              </p>
              <div className="grid gap-4 md:grid-cols-2">
                <button
                  type="button"
                  onClick={() => form.setFieldValue('type', 'basic')}
                  className={`rounded-lg border-2 p-4 text-left transition-colors ${
                    form.state.values.type === 'basic'
                      ? 'border-primary bg-primary/5'
                      : 'border-border hover:border-primary/50'
                  }`}
                >
                  <Shield className="mb-2 h-8 w-8 text-green-500" />
                  <h3 className="font-medium">Basic (Confidential)</h3>
                  <p className="mt-1 text-xs text-muted-foreground">
                    Para aplicações server-side que podem guardar um secret de forma segura.
                  </p>
                </button>
                <button
                  type="button"
                  onClick={() => form.setFieldValue('type', 'public')}
                  className={`rounded-lg border-2 p-4 text-left transition-colors ${
                    form.state.values.type === 'public'
                      ? 'border-primary bg-primary/5'
                      : 'border-border hover:border-primary/50'
                  }`}
                >
                  <Globe className="mb-2 h-8 w-8 text-blue-500" />
                  <h3 className="font-medium">Public (PKCE-only)</h3>
                  <p className="mt-1 text-xs text-muted-foreground">
                    Para SPAs, mobile apps e CLIs que não podem armazenar secrets.
                  </p>
                </button>
              </div>
            </div>
          )}

          {step === 1 && (
            <div className="space-y-4">
              <form.Field name="name">
                {(field) => (
                  <div className="space-y-2">
                    <Label htmlFor="name">Client ID (slug) *</Label>
                    <Input
                      id="name"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                      placeholder="meu-app-web"
                    />
                    <p className="text-xs text-muted-foreground">
                      Letras minúsculas, números e hífens
                    </p>
                  </div>
                )}
              </form.Field>

              <form.Field name="displayname">
                {(field) => (
                  <div className="space-y-2">
                    <Label htmlFor="displayname">Nome de Exibição *</Label>
                    <Input
                      id="displayname"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                      placeholder="Meu App Web"
                    />
                  </div>
                )}
              </form.Field>

              <form.Field name="origin_landing">
                {(field) => (
                  <div className="space-y-2">
                    <Label htmlFor="landing">Landing URL *</Label>
                    <Input
                      id="landing"
                      type="url"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                      placeholder="https://meuapp.exemplo.com"
                    />
                  </div>
                )}
              </form.Field>

              <div className="space-y-2">
                <Label>Redirect URLs</Label>
                <div className="flex gap-2">
                  <Input
                    value={redirectInput}
                    onChange={(e) => setRedirectInput(e.target.value)}
                    placeholder="https://meuapp.exemplo.com/callback"
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.preventDefault()
                        addRedirectUrl()
                      }
                    }}
                  />
                  <Button type="button" variant="outline" size="icon" onClick={addRedirectUrl}>
                    <Plus className="h-4 w-4" />
                  </Button>
                </div>
                <div className="flex flex-wrap gap-2">
                  {form.state.values.redirect_urls.map((url) => (
                    <Badge key={url} variant="secondary" className="gap-1">
                      <span className="max-w-[200px] truncate">{url}</span>
                      <button
                        type="button"
                        onClick={() =>
                          form.setFieldValue(
                            'redirect_urls',
                            form.getFieldValue('redirect_urls').filter((u) => u !== url),
                          )
                        }
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </Badge>
                  ))}
                </div>
              </div>
            </div>
          )}

          {step === 2 && (
            <div className="space-y-4">
              <p className="text-sm text-muted-foreground">
                Configure quais scopes cada grupo terá acesso (opcional)
              </p>
              <div className="grid gap-2 md:grid-cols-[1fr_1fr_auto]">
                <select
                  className="rounded-md border bg-background px-3 py-2 text-sm"
                  value={scopeGroupId}
                  onChange={(e) => setScopeGroupId(e.target.value)}
                >
                  <option value="">Selecione um grupo</option>
                  {groups
                    ?.filter((g) => !g.isBuiltin)
                    .map((g) => (
                      <option key={g.id} value={g.id}>
                        {g.name}
                      </option>
                    ))}
                </select>
                <Input
                  value={scopeInput}
                  onChange={(e) => setScopeInput(e.target.value)}
                  placeholder="openid, profile, email"
                />
                <Button variant="outline" onClick={addScopeMap}>
                  <Plus className="mr-2 h-4 w-4" />
                  Adicionar
                </Button>
              </div>

              {scopeMaps.length > 0 && (
                <div className="space-y-2">
                  {scopeMaps.map((sm) => (
                    <div
                      key={sm.groupId}
                      className="flex items-center justify-between rounded-lg border p-3"
                    >
                      <div>
                        <p className="text-sm font-medium">{sm.groupName}</p>
                        <div className="flex gap-1">
                          {sm.scopes.map((s) => (
                            <Badge key={s} variant="outline" className="text-xs">
                              {s}
                            </Badge>
                          ))}
                        </div>
                      </div>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8"
                        onClick={() =>
                          setScopeMaps((prev) =>
                            prev.filter((m) => m.groupId !== sm.groupId),
                          )
                        }
                      >
                        <X className="h-4 w-4" />
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {step === 3 && (
            <div className="space-y-4">
              <CardHeader className="px-0 pt-0">
                <CardTitle className="text-base">
                  Revise antes de criar
                </CardTitle>
              </CardHeader>
              <div className="space-y-3">
                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <p className="text-muted-foreground">Tipo</p>
                    <p className="font-medium">
                      {form.state.values.type === 'basic' ? 'Basic (Confidential)' : 'Public (PKCE)'}
                    </p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">Client ID</p>
                    <p className="font-mono font-medium">{form.state.values.name}</p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">Nome</p>
                    <p className="font-medium">{form.state.values.displayname}</p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">Landing URL</p>
                    <p className="truncate font-medium">{form.state.values.origin_landing}</p>
                  </div>
                </div>
                {form.state.values.redirect_urls.length > 0 && (
                  <>
                    <Separator />
                    <div>
                      <p className="text-sm text-muted-foreground mb-1">Redirect URLs</p>
                      {form.state.values.redirect_urls.map((url) => (
                        <p key={url} className="text-xs font-mono">{url}</p>
                      ))}
                    </div>
                  </>
                )}
                {scopeMaps.length > 0 && (
                  <>
                    <Separator />
                    <div>
                      <p className="text-sm text-muted-foreground mb-1">Scope Maps</p>
                      {scopeMaps.map((sm) => (
                        <div key={sm.groupId} className="text-sm">
                          <span className="font-medium">{sm.groupName}</span>:{' '}
                          {sm.scopes.join(', ')}
                        </div>
                      ))}
                    </div>
                  </>
                )}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <div className="flex justify-between">
        <Button
          variant="outline"
          onClick={() => {
            if (step === 0) navigate({ to: '/oauth2' })
            else setStep(step - 1)
          }}
        >
          <ArrowLeft className="mr-2 h-4 w-4" />
          {step === 0 ? 'Cancelar' : 'Voltar'}
        </Button>
        {step < STEPS.length - 1 ? (
          <Button onClick={() => setStep(step + 1)} disabled={!canAdvance()}>
            Próximo
            <ArrowRight className="ml-2 h-4 w-4" />
          </Button>
        ) : (
          <Button
            onClick={() => form.handleSubmit()}
            disabled={createClient.isPending}
          >
            {createClient.isPending ? 'Criando...' : 'Criar Client'}
            <Check className="ml-2 h-4 w-4" />
          </Button>
        )}
      </div>
    </div>
  )
}
