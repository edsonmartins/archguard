// src/components/group/group-create-page.tsx

import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useForm } from '@tanstack/react-form'
import { ArrowLeft, Search } from 'lucide-react'
import { Link } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { useCreateGroup, useAddGroupMembers } from '@/lib/hooks/use-groups'
import { usePersons } from '@/lib/hooks/use-persons'
import { createGroupSchema } from '@/lib/utils/validators'

export function GroupCreatePage() {
  const navigate = useNavigate()
  const createGroup = useCreateGroup()
  const addMembers = useAddGroupMembers()
  const { data: persons } = usePersons()
  const [memberSearch, setMemberSearch] = useState('')
  const [selectedMembers, setSelectedMembers] = useState<string[]>([])

  const form = useForm({
    defaultValues: {
      name: '',
      description: '',
    },
    onSubmit: async ({ value }) => {
      createGroup.mutate(
        {
          name: value.name,
          description: value.description || undefined,
        },
        {
          onSuccess: (result) => {
            if (selectedMembers.length > 0) {
              const groupId =
                typeof result === 'object' && result !== null && 'id' in result
                  ? (result as { id: string }).id
                  : value.name
              addMembers.mutate(
                { id: groupId, memberIds: selectedMembers },
                { onSettled: () => navigate({ to: '/groups' }) },
              )
            } else {
              navigate({ to: '/groups' })
            }
          },
        },
      )
    },
  })

  const filteredPersons = persons?.filter(
    (p) =>
      p.displayName.toLowerCase().includes(memberSearch.toLowerCase()) ||
      p.username.toLowerCase().includes(memberSearch.toLowerCase()),
  )

  const toggleMember = (id: string) => {
    setSelectedMembers((prev) =>
      prev.includes(id) ? prev.filter((m) => m !== id) : [...prev, id],
    )
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/groups">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Novo Grupo</h1>
          <p className="text-muted-foreground">
            Crie um grupo para organizar identidades
          </p>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Informações do Grupo</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <form.Field
            name="name"
            validators={{
              onChange: ({ value }) => {
                const r = createGroupSchema.shape.name.safeParse(value)
                return r.success ? undefined : r.error.issues[0]?.message
              },
            }}
          >
            {(field) => (
              <div className="space-y-2">
                <Label htmlFor="name">Nome do Grupo *</Label>
                <Input
                  id="name"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                  placeholder="equipe_dev"
                />
                {field.state.meta.errors.length > 0 && (
                  <p className="text-sm text-destructive">
                    {field.state.meta.errors[0]}
                  </p>
                )}
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
                  placeholder="Descrição opcional do grupo"
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
            Membros Iniciais ({selectedMembers.length})
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={memberSearch}
              onChange={(e) => setMemberSearch(e.target.value)}
              placeholder="Buscar pessoas..."
              className="pl-10"
            />
          </div>

          {selectedMembers.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {selectedMembers.map((id) => {
                const person = persons?.find((p) => p.id === id)
                return (
                  <Badge key={id} variant="secondary">
                    {person?.displayName ?? id}
                  </Badge>
                )
              })}
            </div>
          )}

          <div className="max-h-48 space-y-2 overflow-y-auto">
            {filteredPersons?.slice(0, 20).map((person) => (
              <div
                key={person.id}
                className="flex items-center gap-3 rounded-lg border p-3"
              >
                <Checkbox
                  checked={selectedMembers.includes(person.id)}
                  onCheckedChange={() => toggleMember(person.id)}
                />
                <div className="flex-1">
                  <p className="text-sm font-medium">{person.displayName}</p>
                  <p className="text-xs text-muted-foreground">
                    @{person.username}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <div className="flex justify-between">
        <Button variant="outline" asChild>
          <Link to="/groups">Cancelar</Link>
        </Button>
        <Button
          onClick={() => form.handleSubmit()}
          disabled={createGroup.isPending || addMembers.isPending}
        >
          {createGroup.isPending ? 'Criando...' : 'Criar Grupo'}
        </Button>
      </div>
    </div>
  )
}
