// src/components/group/group-detail-page.tsx

import { useState } from 'react'
import { useNavigate, Link } from '@tanstack/react-router'
import { Route } from '@/routes/_authed/groups/$groupId'
import {
  ArrowLeft,
  Users,
  Settings,
  Trash2,
  Plus,
  X,
  Lock,
  Search,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { PermissionGate } from '@/components/shared/permission-gate'
import { GroupBadge } from '@/components/shared/group-badge'
import { CopyButton } from '@/components/shared/copy-button'
import {
  useGroup,
  useDeleteGroup,
  useAddGroupMembers,
  useRemoveGroupMembers,
} from '@/lib/hooks/use-groups'
import { usePersons } from '@/lib/hooks/use-persons'
import { initials } from '@/lib/utils/formatters'

export function GroupDetailPage() {
  const { groupId } = Route.useParams()
  const navigate = useNavigate()
  const { data: group, isLoading } = useGroup(groupId)
  const { data: allPersons } = usePersons()
  const deleteGroup = useDeleteGroup()
  const addMembers = useAddGroupMembers()
  const removeMembers = useRemoveGroupMembers()

  const [showDelete, setShowDelete] = useState(false)
  const [showAddMembers, setShowAddMembers] = useState(false)
  const [memberSearch, setMemberSearch] = useState('')
  const [selectedToAdd, setSelectedToAdd] = useState<string[]>([])

  if (isLoading || !group) {
    return <GroupDetailSkeleton />
  }

  const memberIds = group.members.map((m) => m.id)
  const availablePersons = allPersons?.filter(
    (p) =>
      !memberIds.includes(p.id) &&
      (p.displayName.toLowerCase().includes(memberSearch.toLowerCase()) ||
        p.username.toLowerCase().includes(memberSearch.toLowerCase())),
  )

  const handleAddMembers = () => {
    addMembers.mutate(
      { id: groupId, memberIds: selectedToAdd },
      {
        onSuccess: () => {
          setShowAddMembers(false)
          setSelectedToAdd([])
          setMemberSearch('')
        },
      },
    )
  }

  const handleRemoveMember = (memberId: string) => {
    removeMembers.mutate({ id: groupId, memberIds: [memberId] })
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/groups">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <h1 className="text-2xl font-bold">{group.name}</h1>
            <GroupBadge name={group.name} />
            {group.isBuiltin && (
              <Badge variant="outline">
                <Lock className="mr-1 h-3 w-3" />
                Sistema
              </Badge>
            )}
          </div>
          {group.description && (
            <p className="text-muted-foreground">{group.description}</p>
          )}
        </div>
        {!group.isBuiltin && (
          <PermissionGate require="groups:delete">
            <Button
              variant="destructive"
              onClick={() => setShowDelete(true)}
            >
              <Trash2 className="mr-2 h-4 w-4" />
              Excluir
            </Button>
          </PermissionGate>
        )}
      </div>

      <Tabs defaultValue="members">
        <TabsList>
          <TabsTrigger value="members">
            <Users className="mr-2 h-4 w-4" />
            Membros ({group.memberCount})
          </TabsTrigger>
          <TabsTrigger value="config">
            <Settings className="mr-2 h-4 w-4" />
            Configuração
          </TabsTrigger>
        </TabsList>

        <TabsContent value="members" className="space-y-4">
          <div className="flex items-center justify-between">
            <p className="text-sm text-muted-foreground">
              {group.memberCount} membro(s) neste grupo
            </p>
            {!group.isBuiltin && (
              <PermissionGate require="groups:members">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setShowAddMembers(true)}
                >
                  <Plus className="mr-2 h-4 w-4" />
                  Adicionar Membros
                </Button>
              </PermissionGate>
            )}
          </div>

          {group.members.length === 0 ? (
            <Card>
              <CardContent className="py-8 text-center">
                <Users className="mx-auto mb-2 h-8 w-8 text-muted-foreground/50" />
                <p className="text-sm text-muted-foreground">
                  Este grupo não possui membros
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="space-y-2">
              {group.members.map((member) => (
                <div
                  key={member.id}
                  className="flex items-center justify-between rounded-lg border p-3"
                >
                  <div className="flex items-center gap-3">
                    <Avatar className="h-8 w-8">
                      <AvatarFallback className="text-xs">
                        {initials(member.displayName ?? member.name)}
                      </AvatarFallback>
                    </Avatar>
                    <div>
                      <p className="text-sm font-medium">
                        {member.displayName ?? member.name}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        @{member.name} · {member.type}
                      </p>
                    </div>
                  </div>
                  {!group.isBuiltin && (
                    <PermissionGate require="groups:members">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-muted-foreground hover:text-destructive"
                        onClick={() => handleRemoveMember(member.id)}
                        disabled={removeMembers.isPending}
                      >
                        <X className="h-4 w-4" />
                      </Button>
                    </PermissionGate>
                  )}
                </div>
              ))}
            </div>
          )}
        </TabsContent>

        <TabsContent value="config">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Informações do Grupo</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <InfoRow label="ID" value={group.id}>
                <CopyButton value={group.id} />
              </InfoRow>
              <InfoRow label="Nome" value={group.name} />
              <InfoRow
                label="Descrição"
                value={group.description ?? 'Sem descrição'}
              />
              <InfoRow label="Tipo" value="">
                <GroupBadge name={group.name} />
              </InfoRow>
              <InfoRow
                label="Builtin"
                value={group.isBuiltin ? 'Sim' : 'Não'}
              />
              <InfoRow label="Membros" value={String(group.memberCount)} />
              {group.memberOf.length > 0 && (
                <div className="text-sm">
                  <span className="text-muted-foreground">Membro de: </span>
                  <div className="mt-1 flex flex-wrap gap-1">
                    {group.memberOf.map((g) => (
                      <Badge key={g} variant="outline">
                        {g}
                      </Badge>
                    ))}
                  </div>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Add Members Dialog */}
      <Dialog open={showAddMembers} onOpenChange={setShowAddMembers}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Adicionar Membros</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={memberSearch}
                onChange={(e) => setMemberSearch(e.target.value)}
                placeholder="Buscar pessoas..."
                className="pl-10"
              />
            </div>
            <div className="max-h-64 space-y-2 overflow-y-auto">
              {availablePersons?.slice(0, 20).map((person) => {
                const toggle = () =>
                  setSelectedToAdd((prev) =>
                    prev.includes(person.id)
                      ? prev.filter((id) => id !== person.id)
                      : [...prev, person.id],
                  )
                return (
                  <button
                    type="button"
                    key={person.id}
                    onClick={toggle}
                    className="flex w-full items-center gap-3 rounded-lg border p-3 text-left hover:bg-accent"
                  >
                    <Checkbox
                      checked={selectedToAdd.includes(person.id)}
                      onCheckedChange={toggle}
                      onClick={(e) => e.stopPropagation()}
                    />
                    <div className="flex-1">
                      <p className="text-sm font-medium">
                        {person.displayName}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        @{person.username}
                      </p>
                    </div>
                  </button>
                )
              })}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowAddMembers(false)}>
              Cancelar
            </Button>
            <Button
              aria-label="Adicionar"
              onClick={handleAddMembers}
              disabled={selectedToAdd.length === 0 || addMembers.isPending}
            >
              {addMembers.isPending ? 'Adicionando...' : 'Adicionar'}
              {selectedToAdd.length > 0 && (
                <Badge variant="secondary" className="ml-2">
                  {selectedToAdd.length}
                </Badge>
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation */}
      <ConfirmDialog
        open={showDelete}
        onOpenChange={setShowDelete}
        title="Excluir Grupo"
        description={`Esta ação é irreversível. O grupo "${group.name}" será permanentemente removido.`}
        confirmText={group.name}
        destructive
        isLoading={deleteGroup.isPending}
        onConfirm={() => {
          deleteGroup.mutate(group.id, {
            onSuccess: () => navigate({ to: '/groups' }),
          })
        }}
      />
    </div>
  )
}

function InfoRow({
  label,
  value,
  children,
}: {
  label: string
  value: string
  children?: React.ReactNode
}) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-muted-foreground">{label}</span>
      <div className="flex items-center gap-2">
        {value && <span className="font-medium">{value}</span>}
        {children}
      </div>
    </div>
  )
}

function GroupDetailSkeleton() {
  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Skeleton className="h-10 w-10" />
        <div className="flex-1">
          <Skeleton className="h-8 w-48" />
          <Skeleton className="mt-1 h-4 w-64" />
        </div>
      </div>
      <Skeleton className="h-10 w-64" />
      <div className="space-y-2">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-14" />
        ))}
      </div>
    </div>
  )
}
