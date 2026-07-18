import { useTranslation } from 'react-i18next'
// src/components/service-account/sa-create-page.tsx

import { useState } from 'react'
import { useNavigate, Link } from '@tanstack/react-router'
import { useForm } from '@tanstack/react-form'
import { ArrowLeft, Search } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { useQueryClient } from '@tanstack/react-query'
import { useCreateServiceAccount } from '@/lib/hooks/use-service-accounts'
import { useGroups } from '@/lib/hooks/use-groups'
import { queryKeys } from '@/lib/utils/query-keys'
import { createServiceAccountSchema } from '@/lib/utils/validators'

export function ServiceAccountCreatePage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const createSA = useCreateServiceAccount()
  const { data: groups } = useGroups()
  const [groupSearch, setGroupSearch] = useState('')
  const [selectedGroups, setSelectedGroups] = useState<string[]>([])

  const form = useForm({
    defaultValues: {
      name: '',
      displayname: '',
      description: '',
    },
    onSubmit: async ({ value }) => {
      await createSA.mutateAsync({
        name: value.name,
        displayname: value.displayname,
        description: value.description || undefined,
        groups: selectedGroups.length > 0 ? selectedGroups : undefined,
      })
      queryClient.removeQueries({ queryKey: queryKeys.serviceAccounts.all })
      navigate({ to: '/service-accounts' })
    },
  })

  const filteredGroups = groups?.filter(
    (g) =>
      !g.isBuiltin &&
      g.name.toLowerCase().includes(groupSearch.toLowerCase()),
  )

  const toggleGroup = (name: string) => {
    setSelectedGroups((prev) =>
      prev.includes(name)
        ? prev.filter((g) => g !== name)
        : [...prev, name],
    )
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/service-accounts">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div>
          <h1 className="text-3xl font-bold tracking-tight">
            Novo Service Account
          </h1>
          <p className="text-muted-foreground">
            Crie uma conta de serviço para integrações M2M
          </p>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Informações</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <form.Field
            name="name"
            validators={{
              onChange: ({ value }) => {
                const r = createServiceAccountSchema.shape.name.safeParse(value)
                return r.success ? undefined : r.error.issues[0]?.message
              },
            }}
          >
            {(field) => (
              <div className="space-y-2">
                <Label htmlFor="name">Nome (SPN) *</Label>
                <Input
                  id="name"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                  placeholder="api-gateway-sa"
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
                const r = createServiceAccountSchema.shape.displayname.safeParse(value)
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
                  placeholder="API Gateway Service Account"
                />
              </div>
            )}
          </form.Field>

          <form.Field name="description">
            {(field) => (
              <div className="space-y-2">
                <Label htmlFor="description">Descrição</Label>
                <Textarea
                  id="description"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                  placeholder={t('serviceAccounts.descriptionPlaceholder')}
                  rows={3}
                />
              </div>
            )}
          </form.Field>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">
            Grupos ({selectedGroups.length})
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={groupSearch}
              onChange={(e) => setGroupSearch(e.target.value)}
              placeholder={t('serviceAccounts.searchGroups')}
              className="pl-10"
            />
          </div>
          {selectedGroups.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {selectedGroups.map((g) => (
                <Badge key={g} variant="secondary">
                  {g}
                </Badge>
              ))}
            </div>
          )}
          <div className="max-h-48 space-y-2 overflow-y-auto">
            {filteredGroups?.slice(0, 15).map((group) => (
              <div
                key={group.id}
                className="flex items-center gap-3 rounded-lg border p-3"
              >
                <Checkbox
                  checked={selectedGroups.includes(group.name)}
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
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <div className="flex justify-between">
        <Button variant="outline" asChild>
          <Link to="/service-accounts">Cancelar</Link>
        </Button>
        <Button
          onClick={() => form.handleSubmit()}
          disabled={createSA.isPending}
        >
          {createSA.isPending ? 'Criando...' : t('serviceAccounts.createSa')}
        </Button>
      </div>
    </div>
  )
}
