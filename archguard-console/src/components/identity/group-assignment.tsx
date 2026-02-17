// src/components/identity/group-assignment.tsx

import { useState } from 'react'
import { Plus, X, Search } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Checkbox } from '@/components/ui/checkbox'
import { PermissionGate } from '@/components/shared/permission-gate'
import { GroupBadge } from '@/components/shared/group-badge'
import { useGroups, useAddGroupMembers, useRemoveGroupMembers } from '@/lib/hooks/use-groups'
import type { Person } from '@/lib/api/types/kanidm'

interface PersonGroupAssignmentProps {
  personId: string
  person: Person
}

export function PersonGroupAssignment({
  personId,
  person,
}: PersonGroupAssignmentProps) {
  const { data: allGroups } = useGroups()
  const addMembers = useAddGroupMembers()
  const removeMembers = useRemoveGroupMembers()
  const [showAddDialog, setShowAddDialog] = useState(false)
  const [groupSearch, setGroupSearch] = useState('')
  const [selectedGroups, setSelectedGroups] = useState<string[]>([])

  const availableGroups = allGroups?.filter(
    (g) =>
      !g.isBuiltin &&
      !person.groupNames.includes(g.name) &&
      g.name.toLowerCase().includes(groupSearch.toLowerCase()),
  )

  const handleAddGroups = () => {
    const promises = selectedGroups.map((groupId) =>
      addMembers.mutateAsync({ id: groupId, memberIds: [personId] }),
    )
    Promise.all(promises).then(() => {
      setShowAddDialog(false)
      setSelectedGroups([])
      setGroupSearch('')
    })
  }

  const handleRemoveFromGroup = (groupName: string) => {
    const group = allGroups?.find((g) => g.name === groupName)
    if (group) {
      removeMembers.mutate({ id: group.id, memberIds: [personId] })
    }
  }

  const toggleGroupSelection = (groupId: string) => {
    setSelectedGroups((prev) =>
      prev.includes(groupId)
        ? prev.filter((id) => id !== groupId)
        : [...prev, groupId],
    )
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-base">
            Grupos ({person.groupNames.length})
          </CardTitle>
          <PermissionGate require="groups:manage">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowAddDialog(true)}
            >
              <Plus className="mr-2 h-4 w-4" />
              Adicionar a Grupo
            </Button>
          </PermissionGate>
        </CardHeader>
        <CardContent>
          {person.groupNames.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              Esta pessoa não pertence a nenhum grupo
            </p>
          ) : (
            <div className="space-y-2">
              {person.groupNames.map((groupName) => (
                <div
                  key={groupName}
                  className="flex items-center justify-between rounded-lg border p-3"
                >
                  <GroupBadge name={groupName} />
                  <PermissionGate require="groups:manage">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-muted-foreground hover:text-destructive"
                      onClick={() => handleRemoveFromGroup(groupName)}
                      disabled={removeMembers.isPending}
                    >
                      <X className="h-4 w-4" />
                    </Button>
                  </PermissionGate>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={showAddDialog} onOpenChange={setShowAddDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Adicionar a Grupos</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={groupSearch}
                onChange={(e) => setGroupSearch(e.target.value)}
                placeholder="Buscar grupos..."
                className="pl-10"
              />
            </div>
            <div className="max-h-64 space-y-2 overflow-y-auto">
              {availableGroups?.map((group) => (
                <div
                  key={group.id}
                  className="flex items-center gap-3 rounded-lg border p-3"
                >
                  <Checkbox
                    checked={selectedGroups.includes(group.id)}
                    onCheckedChange={() => toggleGroupSelection(group.id)}
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
              {availableGroups?.length === 0 && (
                <p className="py-4 text-center text-sm text-muted-foreground">
                  Nenhum grupo disponível
                </p>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowAddDialog(false)}>
              Cancelar
            </Button>
            <Button
              onClick={handleAddGroups}
              disabled={selectedGroups.length === 0 || addMembers.isPending}
            >
              {addMembers.isPending
                ? 'Adicionando...'
                : `Adicionar (${selectedGroups.length})`}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
