// W-C3 — Active Warpgate sessions + kill

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, RefreshCw, Skull } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Skeleton } from '@/components/ui/skeleton'
import {
  listWarpgateSessionsFn,
  terminateWarpgateSessionFn,
} from '@/server/warpgate-fn'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { useState } from 'react'

type Props = {
  canRead: boolean
  canManage: boolean
}

export function SessionsPanel({ canRead, canManage }: Props) {
  const qc = useQueryClient()
  const [killId, setKillId] = useState<string | null>(null)

  const q = useQuery({
    queryKey: ['warpgate', 'sessions'],
    queryFn: () => listWarpgateSessionsFn(),
    enabled: canRead,
    refetchInterval: 15_000,
  })

  const kill = useMutation({
    mutationFn: (id: string) => terminateWarpgateSessionFn({ data: { id } }),
    onSuccess: () => {
      toast.success('Sessão encerrada')
      setKillId(null)
      void qc.invalidateQueries({ queryKey: ['warpgate', 'sessions'] })
    },
    onError: (e) => toast.error((e as Error).message),
  })

  if (!canRead) {
    return (
      <p className="text-sm text-muted-foreground">Sem permissão para sessões.</p>
    )
  }

  if (q.isLoading) return <Skeleton className="h-40 w-full" />

  const sessions = q.data?.sessions || []
  const err = q.data?.error

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-2">
          <div>
            <CardTitle className="flex items-center gap-2 text-base">
              <Activity className="h-5 w-5" />
              Sessões ativas (Warpgate)
            </CardTitle>
            <CardDescription>
              Encerrar sessão de operador sem abrir a UI stock do bastion.
              Atualiza a cada 15s.
            </CardDescription>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => void q.refetch()}
            disabled={q.isFetching}
          >
            <RefreshCw className="h-4 w-4 mr-1" />
            Atualizar
          </Button>
        </CardHeader>
        <CardContent>
          {err && (
            <p className="text-sm text-amber-700 mb-3">
              API de sessões: {err} (versão Warpgate pode não expor este endpoint —
              break-glass: UI stock).
            </p>
          )}
          {sessions.length === 0 && !err && (
            <p className="text-sm text-muted-foreground">Nenhuma sessão ativa.</p>
          )}
          {sessions.length > 0 && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>Usuário</TableHead>
                  <TableHead>Target</TableHead>
                  <TableHead>Início</TableHead>
                  <TableHead>Origem</TableHead>
                  <TableHead />
                </TableRow>
              </TableHeader>
              <TableBody>
                {sessions.map((s) => (
                  <TableRow key={s.id}>
                    <TableCell className="font-mono text-xs max-w-[120px] truncate">
                      {s.id}
                    </TableCell>
                    <TableCell>{s.username || '—'}</TableCell>
                    <TableCell>
                      <Badge variant="outline">{s.target || '—'}</Badge>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {s.started_at
                        ? new Date(s.started_at).toLocaleString()
                        : '—'}
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      {s.address || '—'}
                    </TableCell>
                    <TableCell className="text-right">
                      {canManage && (
                        <Button
                          size="sm"
                          variant="destructive"
                          onClick={() => setKillId(s.id)}
                        >
                          <Skull className="h-3.5 w-3.5 mr-1" />
                          Kill
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <ConfirmDialog
        open={!!killId}
        onOpenChange={(o) => !o && setKillId(null)}
        title="Encerrar sessão"
        description={`Encerra a sessão ${killId} no Warpgate. O operador perde a conexão imediatamente. Digite o ID para confirmar.`}
        confirmText={killId || ''}
        destructive
        isLoading={kill.isPending}
        onConfirm={() => {
          if (killId) kill.mutate(killId)
        }}
      />
    </div>
  )
}
