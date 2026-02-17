// src/components/audit/audit-page.tsx

import { useState, useMemo } from 'react'
import {
  useReactTable,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  getPaginationRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
} from '@tanstack/react-table'
import {
  ClipboardList,
  Search,
  Download,
  Filter,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  Info,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Separator } from '@/components/ui/separator'
import { EmptyState } from '@/components/shared/empty-state'
import { TimeAgo } from '@/components/shared/time-ago'
import { useAuditEvents } from '@/lib/hooks/use-audit'
import type { AuditEvent, AuditEventType } from '@/lib/api/types/kanidm'
import type { AuditFilters } from '@/lib/utils/validators'

const EVENT_TYPE_LABELS: Record<AuditEventType, string> = {
  auth_success: 'Login OK',
  auth_failure: 'Login Falhou',
  person_created: 'Pessoa Criada',
  person_updated: 'Pessoa Atualizada',
  person_deleted: 'Pessoa Removida',
  group_created: 'Grupo Criado',
  group_updated: 'Grupo Atualizado',
  group_member_added: 'Membro Adicionado',
  group_member_removed: 'Membro Removido',
  oauth2_client_created: 'OAuth2 Criado',
  oauth2_client_updated: 'OAuth2 Atualizado',
  credential_reset: 'Reset Credencial',
  account_locked: 'Conta Bloqueada',
  account_unlocked: 'Conta Desbloqueada',
  token_generated: 'Token Gerado',
  token_revoked: 'Token Revogado',
}

const EVENT_TYPE_ICONS: Record<string, typeof CheckCircle2> = {
  auth_success: CheckCircle2,
  auth_failure: XCircle,
  account_locked: AlertTriangle,
  account_unlocked: CheckCircle2,
}

export function AuditPage() {
  const [filters, setFilters] = useState<AuditFilters>({
    period: '24h',
    status: ['success', 'failure', 'alert'],
  })
  const [sorting, setSorting] = useState<SortingState>([
    { id: 'timestamp', desc: true },
  ])
  const [globalFilter, setGlobalFilter] = useState('')
  const [selectedEvent, setSelectedEvent] = useState<AuditEvent | null>(null)

  const { data: events, isLoading } = useAuditEvents(filters)

  const columns: ColumnDef<AuditEvent>[] = useMemo(
    () => [
      {
        accessorKey: 'timestamp',
        header: 'Data/Hora',
        cell: ({ row }) => (
          <TimeAgo date={row.original.timestamp} />
        ),
        size: 140,
      },
      {
        accessorKey: 'eventType',
        header: 'Evento',
        cell: ({ row }) => {
          const type = row.original.eventType
          const Icon = EVENT_TYPE_ICONS[type] ?? Info
          const isError = type.includes('failure') || type.includes('locked')
          return (
            <div className="flex items-center gap-2">
              <Icon
                className={`h-4 w-4 ${
                  isError ? 'text-destructive' : 'text-muted-foreground'
                }`}
              />
              <span className="text-sm">
                {EVENT_TYPE_LABELS[type] ?? type}
              </span>
            </div>
          )
        },
      },
      {
        accessorKey: 'actor',
        header: 'Ator',
        cell: ({ row }) => (
          <span className="font-mono text-sm">{row.original.actor}</span>
        ),
      },
      {
        accessorKey: 'target',
        header: 'Alvo',
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground">
            {row.original.target ?? '—'}
          </span>
        ),
      },
      {
        accessorKey: 'sourceIp',
        header: 'IP',
        cell: ({ row }) => (
          <span className="font-mono text-xs text-muted-foreground">
            {row.original.sourceIp ?? '—'}
          </span>
        ),
        size: 120,
      },
      {
        id: 'actions',
        cell: ({ row }) => (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setSelectedEvent(row.original)}
          >
            Detalhes
          </Button>
        ),
        size: 80,
      },
    ],
    [],
  )

  const table = useReactTable({
    data: events ?? [],
    columns,
    state: { sorting, globalFilter },
    onSortingChange: setSorting,
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageSize: 25 } },
  })

  const handleExport = (format: 'csv' | 'json') => {
    if (!events) return
    let content: string
    let type: string
    let ext: string

    if (format === 'json') {
      content = JSON.stringify(events, null, 2)
      type = 'application/json'
      ext = 'json'
    } else {
      const headers = 'timestamp,eventType,actor,target,sourceIp'
      const rows = events.map(
        (e) =>
          `${e.timestamp.toISOString()},${e.eventType},${e.actor},${e.target ?? ''},${e.sourceIp ?? ''}`,
      )
      content = [headers, ...rows].join('\n')
      type = 'text/csv'
      ext = 'csv'
    }

    const blob = new Blob([content], { type })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `audit-${new Date().toISOString().slice(0, 10)}.${ext}`
    a.click()
    URL.revokeObjectURL(url)
  }

  if (isLoading) {
    return <AuditSkeleton />
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Auditoria</h1>
          <p className="text-muted-foreground">
            Visualize eventos e atividades do sistema
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => handleExport('csv')}
          >
            <Download className="mr-2 h-4 w-4" />
            CSV
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => handleExport('json')}
          >
            <Download className="mr-2 h-4 w-4" />
            JSON
          </Button>
        </div>
      </div>

      <div className="flex items-center gap-4">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Buscar por ator ou alvo..."
            value={globalFilter}
            onChange={(e) => setGlobalFilter(e.target.value)}
            className="pl-10"
          />
        </div>
        <Select
          value={filters.period}
          onValueChange={(v) =>
            setFilters({ ...filters, period: v as AuditFilters['period'] })
          }
        >
          <SelectTrigger className="w-[140px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="1h">Última hora</SelectItem>
            <SelectItem value="24h">Últimas 24h</SelectItem>
            <SelectItem value="7d">Últimos 7 dias</SelectItem>
            <SelectItem value="30d">Últimos 30 dias</SelectItem>
          </SelectContent>
        </Select>
        <Select
          value={filters.eventType ?? 'all'}
          onValueChange={(v) =>
            setFilters({
              ...filters,
              eventType: v === 'all' ? undefined : v,
            })
          }
        >
          <SelectTrigger className="w-[180px]">
            <SelectValue placeholder="Tipo de evento" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Todos os tipos</SelectItem>
            <SelectItem value="auth_success">Login OK</SelectItem>
            <SelectItem value="auth_failure">Login Falhou</SelectItem>
            <SelectItem value="person_created">Pessoa Criada</SelectItem>
            <SelectItem value="person_deleted">Pessoa Removida</SelectItem>
            <SelectItem value="credential_reset">Reset Credencial</SelectItem>
            <SelectItem value="account_locked">Conta Bloqueada</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {!events || events.length === 0 ? (
        <EmptyState
          icon={ClipboardList}
          title="Nenhum evento encontrado"
          description="Não há eventos de auditoria para os filtros selecionados."
        />
      ) : (
        <>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                {table.getHeaderGroups().map((headerGroup) => (
                  <TableRow key={headerGroup.id}>
                    {headerGroup.headers.map((header) => (
                      <TableHead key={header.id} style={{ width: header.getSize() }}>
                        {header.isPlaceholder
                          ? null
                          : flexRender(
                              header.column.columnDef.header,
                              header.getContext(),
                            )}
                      </TableHead>
                    ))}
                  </TableRow>
                ))}
              </TableHeader>
              <TableBody>
                {table.getRowModel().rows.map((row) => (
                  <TableRow
                    key={row.id}
                    className="cursor-pointer"
                    onClick={() => setSelectedEvent(row.original)}
                  >
                    {row.getVisibleCells().map((cell) => (
                      <TableCell key={cell.id}>
                        {flexRender(
                          cell.column.columnDef.cell,
                          cell.getContext(),
                        )}
                      </TableCell>
                    ))}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <div className="flex items-center justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => table.previousPage()}
              disabled={!table.getCanPreviousPage()}
            >
              Anterior
            </Button>
            <span className="text-sm text-muted-foreground">
              Página {table.getState().pagination.pageIndex + 1} de{' '}
              {table.getPageCount()}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => table.nextPage()}
              disabled={!table.getCanNextPage()}
            >
              Próxima
            </Button>
          </div>
        </>
      )}

      {/* Event Detail Sheet */}
      <Sheet
        open={!!selectedEvent}
        onOpenChange={(open) => !open && setSelectedEvent(null)}
      >
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Detalhes do Evento</SheetTitle>
          </SheetHeader>
          {selectedEvent && (
            <div className="mt-6 space-y-4">
              <DetailRow
                label="Tipo"
                value={EVENT_TYPE_LABELS[selectedEvent.eventType] ?? selectedEvent.eventType}
              />
              <DetailRow label="ID" value={selectedEvent.id} />
              <DetailRow
                label="Data/Hora"
                value={selectedEvent.timestamp.toLocaleString('pt-BR')}
              />
              <DetailRow label="Ator" value={selectedEvent.actor} />
              <DetailRow label="Alvo" value={selectedEvent.target ?? '—'} />
              <DetailRow label="IP de Origem" value={selectedEvent.sourceIp ?? '—'} />
              <Separator />
              <div>
                <p className="mb-2 text-sm font-medium">Detalhes</p>
                <pre className="rounded-lg border bg-muted p-4 text-xs overflow-auto">
                  {JSON.stringify(selectedEvent.details, null, 2)}
                </pre>
              </div>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  )
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium">{value}</span>
    </div>
  )
}

function AuditSkeleton() {
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <Skeleton className="h-9 w-40" />
          <Skeleton className="mt-2 h-5 w-64" />
        </div>
      </div>
      <div className="flex gap-4">
        <Skeleton className="h-10 w-80" />
        <Skeleton className="h-10 w-36" />
        <Skeleton className="h-10 w-44" />
      </div>
      <div className="rounded-md border">
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} className="flex items-center gap-4 border-b p-4">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-4 w-24" />
          </div>
        ))}
      </div>
    </div>
  )
}
