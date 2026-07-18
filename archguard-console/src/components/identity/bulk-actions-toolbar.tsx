import { useTranslation } from 'react-i18next'
// src/components/identity/bulk-actions-toolbar.tsx

import { useState } from 'react'
import {
  UsersRound,
  UserMinus,
  KeySquare,
  Download,
  X,
  Search,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { PermissionGate } from '@/components/shared/permission-gate'
import { useGroups, useAddGroupMembers, useRemoveGroupMembers } from '@/lib/hooks/use-groups'
import { useResetPersonCredential } from '@/lib/hooks/use-persons'
import type { Person } from '@/lib/api/types/kanidm'

interface BulkActionsToolbarProps {
  selectedPersons: Person[]
  onClearSelection: () => void
}

type BulkAction = 'add-group' | 'remove-group' | 'reset-creds' | 'export'

export function BulkActionsToolbar({
  selectedPersons,
  onClearSelection,
}: BulkActionsToolbarProps) {
  const { t } = useTranslation()
  const { data: groups } = useGroups()
  const addMembers = useAddGroupMembers()
  const removeMembers = useRemoveGroupMembers()
  const resetCredential = useResetPersonCredential()

  const [activeAction, setActiveAction] = useState<BulkAction | null>(null)
  const [selectedGroupId, setSelectedGroupId] = useState('')
  const [groupSearch, setGroupSearch] = useState('')
  const [progress, setProgress] = useState(0)
  const [isProcessing, setIsProcessing] = useState(false)

  if (selectedPersons.length === 0) return null

  const filteredGroups = groups?.filter(
    (g) =>
      !g.isBuiltin &&
      g.name.toLowerCase().includes(groupSearch.toLowerCase()),
  )

  const handleAddToGroup = async () => {
    if (!selectedGroupId) return
    setIsProcessing(true)
    setProgress(0)

    const personIds = selectedPersons.map((p) => p.id)
    try {
      await addMembers.mutateAsync({
        id: selectedGroupId,
        memberIds: personIds,
      })
      setProgress(100)
    } finally {
      setIsProcessing(false)
      setActiveAction(null)
      onClearSelection()
      setSelectedGroupId('')
    }
  }

  const handleRemoveFromGroup = async () => {
    if (!selectedGroupId) return
    setIsProcessing(true)
    setProgress(0)

    const personIds = selectedPersons.map((p) => p.id)
    try {
      await removeMembers.mutateAsync({
        id: selectedGroupId,
        memberIds: personIds,
      })
      setProgress(100)
    } finally {
      setIsProcessing(false)
      setActiveAction(null)
      onClearSelection()
      setSelectedGroupId('')
    }
  }

  const handleBulkResetCredentials = async () => {
    setIsProcessing(true)
    setProgress(0)
    const total = selectedPersons.length

    for (let i = 0; i < total; i++) {
      try {
        await resetCredential.mutateAsync({ id: selectedPersons[i]!.id })
      } catch {
        // Continue on error
      }
      setProgress(((i + 1) / total) * 100)
    }

    setIsProcessing(false)
    setActiveAction(null)
    onClearSelection()
  }

  const handleExportCsv = () => {
    const headers = 'username,displayName,email,status,groups'
    const rows = selectedPersons.map(
      (p) =>
        `${p.username},"${p.displayName}",${p.emails[0] ?? ''},${p.status},"${p.groupNames.join(';')}"`,
    )
    const content = [headers, ...rows].join('\n')
    const blob = new Blob([content], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `persons-export-${new Date().toISOString().slice(0, 10)}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <>
      <div className="flex items-center gap-2 rounded-lg border bg-muted/50 p-2">
        <Badge variant="secondary">{selectedPersons.length} selecionado(s)</Badge>
        <div className="mx-2 h-4 w-px bg-border" />

        <PermissionGate require="groups:members">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setActiveAction('add-group')}
          >
            <UsersRound className="mr-2 h-4 w-4" />
            {t('identities.addToGroup')}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setActiveAction('remove-group')}
          >
            <UserMinus className="mr-2 h-4 w-4" />
            {t('identities.removeFromGroup')}
          </Button>
        </PermissionGate>

        <PermissionGate require="persons:credentials">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setActiveAction('reset-creds')}
          >
            <KeySquare className="mr-2 h-4 w-4" />
            Reset Credenciais
          </Button>
        </PermissionGate>

        <Button variant="ghost" size="sm" onClick={handleExportCsv}>
          <Download className="mr-2 h-4 w-4" />
          Exportar CSV
        </Button>

        <div className="flex-1" />
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          onClick={onClearSelection}
          aria-label={t('identities.clearSelection')}
        >
          <X className="h-4 w-4" />
        </Button>
      </div>

      {/* Group Selection Dialog (Add/Remove) */}
      <Dialog
        open={activeAction === 'add-group' || activeAction === 'remove-group'}
        onOpenChange={(open) => !open && setActiveAction(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {activeAction === 'add-group'
                ? t('identities.addToGroup')
                : t('identities.removeFromGroup')}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            {isProcessing ? (
              <div className="space-y-2">
                <Progress value={progress} />
                <p className="text-center text-sm text-muted-foreground">
                  Processando...
                </p>
              </div>
            ) : (
              <>
                <div className="relative">
                  <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    value={groupSearch}
                    onChange={(e) => setGroupSearch(e.target.value)}
                    placeholder={t('identities.searchGroups')}
                    className="pl-10"
                  />
                </div>
                <div className="max-h-48 space-y-2 overflow-y-auto">
                  {filteredGroups?.map((group) => (
                    <div
                      key={group.id}
                      className={`flex items-center gap-3 rounded-lg border p-3 cursor-pointer ${
                        selectedGroupId === group.id
                          ? 'border-primary bg-primary/5'
                          : ''
                      }`}
                      onClick={() => setSelectedGroupId(group.id)}
                    >
                      <div className="flex-1">
                        <p className="text-sm font-medium">{group.name}</p>
                      </div>
                      <Badge variant="outline" className="text-xs">
                        {group.memberCount} membros
                      </Badge>
                    </div>
                  ))}
                </div>
              </>
            )}
          </div>
          {!isProcessing && (
            <DialogFooter>
              <Button variant="outline" onClick={() => setActiveAction(null)}>
                Cancelar
              </Button>
              <Button
                onClick={
                  activeAction === 'add-group'
                    ? handleAddToGroup
                    : handleRemoveFromGroup
                }
                disabled={!selectedGroupId}
              >
                {activeAction === 'add-group' ? 'Adicionar' : 'Remover'}{' '}
                {selectedPersons.length} pessoas
              </Button>
            </DialogFooter>
          )}
        </DialogContent>
      </Dialog>

      {/* Reset Credentials Confirmation */}
      <Dialog
        open={activeAction === 'reset-creds'}
        onOpenChange={(open) => !open && setActiveAction(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Reset de Credenciais em Lote</DialogTitle>
          </DialogHeader>
          <div className="py-4">
            {isProcessing ? (
              <div className="space-y-2">
                <Progress value={progress} />
                <p className="text-center text-sm text-muted-foreground">
                  Gerando links de reset... {Math.round(progress)}%
                </p>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">
                Será gerado um link de reset de credencial para cada uma das{' '}
                <strong>{selectedPersons.length}</strong> pessoas selecionadas.
                Os links terão validade de 24 horas.
              </p>
            )}
          </div>
          {!isProcessing && (
            <DialogFooter>
              <Button variant="outline" onClick={() => setActiveAction(null)}>
                Cancelar
              </Button>
              <Button onClick={handleBulkResetCredentials}>
                Gerar {selectedPersons.length} Links de Reset
              </Button>
            </DialogFooter>
          )}
        </DialogContent>
      </Dialog>
    </>
  )
}
