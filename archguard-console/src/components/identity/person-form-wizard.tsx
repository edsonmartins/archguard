// src/components/identity/person-form-wizard.tsx

import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useForm } from '@tanstack/react-form'
import {
  ArrowLeft,
  ArrowRight,
  Check,
  User,
  UsersRound,
  ClipboardCheck,
  Plus,
  X,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Separator } from '@/components/ui/separator'
import { useCreatePerson } from '@/lib/hooks/use-persons'
import { useGroups } from '@/lib/hooks/use-groups'
import { createPersonSchema } from '@/lib/utils/validators'
import type { CreatePersonInput } from '@/lib/utils/validators'

const STEPS = [
  { id: 'basic', label: 'Dados Básicos', icon: User },
  { id: 'groups', label: 'Grupos', icon: UsersRound },
  { id: 'review', label: 'Revisão', icon: ClipboardCheck },
] as const

export function PersonFormWizard() {
  const navigate = useNavigate()
  const createPerson = useCreatePerson()
  const { data: groups } = useGroups()
  const [step, setStep] = useState(0)
  const [emailInput, setEmailInput] = useState('')
  const [groupSearch, setGroupSearch] = useState('')

  const form = useForm({
    defaultValues: {
      name: '',
      displayname: '',
      legalname: '',
      mail: [] as string[],
      groups: [] as string[],
    } satisfies CreatePersonInput,
    onSubmit: async ({ value }) => {
      createPerson.mutate(
        {
          name: value.name,
          displayname: value.displayname,
          legalname: value.legalname || undefined,
          mail: value.mail,
          groups: value.groups.length > 0 ? value.groups : undefined,
        },
        {
          onSuccess: () => {
            navigate({ to: '/identities' })
          },
        },
      )
    },
  })

  const canAdvance = () => {
    if (step === 0) {
      const values = form.state.values
      return (
        values.name.length >= 2 &&
        values.displayname.length >= 1 &&
        values.mail.length >= 1
      )
    }
    return true
  }

  const addEmail = () => {
    const email = emailInput.trim()
    if (email && /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      const current = form.getFieldValue('mail')
      if (!current.includes(email)) {
        form.setFieldValue('mail', [...current, email])
      }
      setEmailInput('')
    }
  }

  const removeEmail = (email: string) => {
    const current = form.getFieldValue('mail')
    form.setFieldValue(
      'mail',
      current.filter((e) => e !== email),
    )
  }

  const toggleGroup = (groupName: string) => {
    const current = form.getFieldValue('groups')
    if (current.includes(groupName)) {
      form.setFieldValue(
        'groups',
        current.filter((g) => g !== groupName),
      )
    } else {
      form.setFieldValue('groups', [...current, groupName])
    }
  }

  const filteredGroups = groups?.filter(
    (g) =>
      !g.isBuiltin &&
      g.name.toLowerCase().includes(groupSearch.toLowerCase()),
  )

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Nova Pessoa</h1>
        <p className="text-muted-foreground">
          Crie uma nova identidade no sistema
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
              {i > 0 && (
                <Separator className="w-8" />
              )}
              <div
                className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm ${
                  isActive
                    ? 'bg-primary text-primary-foreground'
                    : isDone
                      ? 'bg-primary/10 text-primary'
                      : 'text-muted-foreground'
                }`}
              >
                {isDone ? (
                  <Check className="h-4 w-4" />
                ) : (
                  <Icon className="h-4 w-4" />
                )}
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
              <form.Field
                name="name"
                validators={{
                  onChange: ({ value }) => {
                    const r = createPersonSchema.shape.name.safeParse(value)
                    return r.success ? undefined : r.error.issues[0]?.message
                  },
                }}
              >
                {(field) => (
                  <div className="space-y-2">
                    <Label htmlFor="name">Username *</Label>
                    <Input
                      id="name"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                      placeholder="joao.silva"
                    />
                    {field.state.meta.errors.length > 0 && (
                      <p className="text-sm text-destructive">
                        {field.state.meta.errors[0]}
                      </p>
                    )}
                  </div>
                )}
              </form.Field>

              <form.Field
                name="displayname"
                validators={{
                  onChange: ({ value }) => {
                    const r = createPersonSchema.shape.displayname.safeParse(value)
                    return r.success ? undefined : r.error.issues[0]?.message
                  },
                }}
              >
                {(field) => (
                  <div className="space-y-2">
                    <Label htmlFor="displayname">Nome de Exibição *</Label>
                    <Input
                      id="displayname"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                      placeholder="João da Silva"
                    />
                    {field.state.meta.errors.length > 0 && (
                      <p className="text-sm text-destructive">
                        {field.state.meta.errors[0]}
                      </p>
                    )}
                  </div>
                )}
              </form.Field>

              <form.Field name="legalname">
                {(field) => (
                  <div className="space-y-2">
                    <Label htmlFor="legalname">Nome Legal</Label>
                    <Input
                      id="legalname"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                      placeholder="João Pedro da Silva (opcional)"
                    />
                  </div>
                )}
              </form.Field>

              <div className="space-y-2">
                <Label>Emails *</Label>
                <div className="flex gap-2">
                  <Input
                    value={emailInput}
                    onChange={(e) => setEmailInput(e.target.value)}
                    placeholder="email@exemplo.com"
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.preventDefault()
                        addEmail()
                      }
                    }}
                  />
                  <Button type="button" variant="outline" size="icon" onClick={addEmail}>
                    <Plus className="h-4 w-4" />
                  </Button>
                </div>
                <div className="flex flex-wrap gap-2">
                  {form.state.values.mail.map((email) => (
                    <Badge key={email} variant="secondary" className="gap-1">
                      {email}
                      <button
                        type="button"
                        onClick={() => removeEmail(email)}
                        className="ml-1 rounded-full hover:bg-muted"
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </Badge>
                  ))}
                </div>
                {form.state.values.mail.length === 0 && (
                  <p className="text-sm text-muted-foreground">
                    Adicione pelo menos um email
                  </p>
                )}
              </div>
            </div>
          )}

          {step === 1 && (
            <div className="space-y-4">
              <div>
                <Label>Grupos</Label>
                <p className="text-sm text-muted-foreground mb-3">
                  Selecione os grupos que esta pessoa deve pertencer
                </p>
                <Input
                  value={groupSearch}
                  onChange={(e) => setGroupSearch(e.target.value)}
                  placeholder="Buscar grupos..."
                  className="mb-3"
                />
              </div>
              <div className="max-h-64 space-y-2 overflow-y-auto">
                {filteredGroups?.map((group) => (
                  <div
                    key={group.id}
                    className="flex items-center gap-3 rounded-lg border p-3"
                  >
                    <Checkbox
                      checked={form.state.values.groups.includes(group.name)}
                      onCheckedChange={() => toggleGroup(group.name)}
                    />
                    <div className="flex-1">
                      <p className="text-sm font-medium">{group.name}</p>
                      {group.description && (
                        <p className="text-xs text-muted-foreground">
                          {group.description}
                        </p>
                      )}
                    </div>
                    <Badge variant="outline" className="text-xs">
                      {group.memberCount} membros
                    </Badge>
                  </div>
                ))}
                {filteredGroups?.length === 0 && (
                  <p className="py-4 text-center text-sm text-muted-foreground">
                    Nenhum grupo encontrado
                  </p>
                )}
              </div>
              {form.state.values.groups.length > 0 && (
                <div className="flex flex-wrap gap-2 pt-2">
                  <span className="text-sm text-muted-foreground">
                    Selecionados:
                  </span>
                  {form.state.values.groups.map((g) => (
                    <Badge key={g} variant="secondary">
                      {g}
                    </Badge>
                  ))}
                </div>
              )}
            </div>
          )}

          {step === 2 && (
            <div className="space-y-4">
              <CardHeader className="px-0 pt-0">
                <CardTitle className="text-base">
                  Revise os dados antes de criar
                </CardTitle>
              </CardHeader>
              <div className="space-y-3">
                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <p className="text-muted-foreground">Username</p>
                    <p className="font-medium">
                      @{form.state.values.name}
                    </p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">Nome de Exibição</p>
                    <p className="font-medium">
                      {form.state.values.displayname}
                    </p>
                  </div>
                  {form.state.values.legalname && (
                    <div>
                      <p className="text-muted-foreground">Nome Legal</p>
                      <p className="font-medium">
                        {form.state.values.legalname}
                      </p>
                    </div>
                  )}
                </div>
                <Separator />
                <div>
                  <p className="text-sm text-muted-foreground mb-1">Emails</p>
                  <div className="flex flex-wrap gap-1">
                    {form.state.values.mail.map((email) => (
                      <Badge key={email} variant="outline">
                        {email}
                      </Badge>
                    ))}
                  </div>
                </div>
                {form.state.values.groups.length > 0 && (
                  <>
                    <Separator />
                    <div>
                      <p className="text-sm text-muted-foreground mb-1">
                        Grupos
                      </p>
                      <div className="flex flex-wrap gap-1">
                        {form.state.values.groups.map((g) => (
                          <Badge key={g} variant="secondary">
                            {g}
                          </Badge>
                        ))}
                      </div>
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
            if (step === 0) {
              navigate({ to: '/identities' })
            } else {
              setStep(step - 1)
            }
          }}
        >
          <ArrowLeft className="mr-2 h-4 w-4" />
          {step === 0 ? 'Cancelar' : 'Voltar'}
        </Button>
        {step < STEPS.length - 1 ? (
          <Button
            onClick={() => setStep(step + 1)}
            disabled={!canAdvance()}
          >
            Próximo
            <ArrowRight className="ml-2 h-4 w-4" />
          </Button>
        ) : (
          <Button
            onClick={() => form.handleSubmit()}
            disabled={createPerson.isPending}
          >
            {createPerson.isPending ? 'Criando...' : 'Criar Pessoa'}
            <Check className="ml-2 h-4 w-4" />
          </Button>
        )}
      </div>
    </div>
  )
}
