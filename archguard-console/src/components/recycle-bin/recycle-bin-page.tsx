// src/components/recycle-bin/recycle-bin-page.tsx

import { useState } from 'react'
import { Trash2, RotateCcw, Search, User, UsersRound, Bot, KeyRound } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { EmptyState } from '@/components/shared/empty-state'
import { useRecycleBin, useReviveEntry } from '@/lib/hooks/use-recycle-bin'
import type { RecycleBinEntry } from '@/lib/api/types/kanidm'

const TYPE_CONFIG: Record<string, { label: string; icon: React.ComponentType<{ className?: string }>; variant: 'default' | 'secondary' | 'outline' }> = {
  person: { label: 'Pessoa', icon: User, variant: 'default' },
  group: { label: 'Grupo', icon: UsersRound, variant: 'secondary' },
  service_account: { label: 'Service Account', icon: Bot, variant: 'outline' },
  oauth2_resource_server: { label: 'OAuth2', icon: KeyRound, variant: 'outline' },
}

export function RecycleBinPage() {
  const { data: entries, isLoading } = useRecycleBin()
  const reviveEntry = useReviveEntry()
  const [search, setSearch] = useState('')
  const [reviveTarget, setReviveTarget] = useState<RecycleBinEntry | null>(null)

  const filtered = (entries ?? []).filter((e) =>
    e.name.toLowerCase().includes(search.toLowerCase()) ||
    e.type.toLowerCase().includes(search.toLowerCase()),
  )

  if (isLoading) {
    return <RecycleBinSkeleton />
  }

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Lixeira</h1>
        <p className="text-muted-foreground">
          Itens excluídos recentemente. Restaure antes que sejam removidos permanentemente.
        </p>
      </div>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Buscar na lixeira..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="pl-10"
        />
      </div>

      {filtered.length === 0 ? (
        <EmptyState
          icon={Trash2}
          title="Lixeira vazia"
          description="Nenhum item excluído encontrado."
        />
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Nome</TableHead>
                <TableHead>Tipo</TableHead>
                <TableHead>UUID</TableHead>
                <TableHead className="w-[100px]">Ações</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((entry) => {
                const config = TYPE_CONFIG[entry.type] ?? { label: entry.type, icon: Trash2, variant: 'outline' as const }
                const Icon = config.icon
                return (
                  <TableRow key={entry.id}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <Icon className="h-4 w-4 text-muted-foreground" />
                        <span className="font-medium">{entry.name}</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={config.variant}>{config.label}</Badge>
                    </TableCell>
                    <TableCell>
                      <span className="font-mono text-xs text-muted-foreground">
                        {entry.id.slice(0, 8)}...
                      </span>
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setReviveTarget(entry)}
                      >
                        <RotateCcw className="mr-1 h-3 w-3" />
                        Restaurar
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}

      <ConfirmDialog
        open={!!reviveTarget}
        onOpenChange={(open) => !open && setReviveTarget(null)}
        title="Restaurar Item"
        description={`Deseja restaurar "${reviveTarget?.name}" (${reviveTarget?.type})? O item será reativado no sistema.`}
        confirmText={reviveTarget?.name ?? ''}
        isLoading={reviveEntry.isPending}
        onConfirm={() => {
          if (reviveTarget) {
            reviveEntry.mutate(reviveTarget.id, {
              onSuccess: () => setReviveTarget(null),
            })
          }
        }}
      />
    </div>
  )
}

function RecycleBinSkeleton() {
  return (
    <div className="space-y-4">
      <div>
        <Skeleton className="h-9 w-36" />
        <Skeleton className="mt-2 h-5 w-80" />
      </div>
      <Skeleton className="h-10 w-80" />
      <div className="rounded-md border">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex items-center gap-4 border-b p-4">
            <Skeleton className="h-4 w-4" />
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-5 w-20" />
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-8 w-24" />
          </div>
        ))}
      </div>
    </div>
  )
}
