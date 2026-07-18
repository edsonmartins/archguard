import { useTranslation } from 'react-i18next'
// Unified audit timeline — console activity + best-effort Warpgate sessions

import { useMemo, useState } from 'react'
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
  CheckCircle2,
  XCircle,
  Info,
  RefreshCw,
} from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
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
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/shared/empty-state'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  listUnifiedAuditFn,
  type TimelineEvent,
} from '@/server/unified-audit-fn'

export function AuditPage() {
  const { t, i18n } = useTranslation()
  const [source, setSource] = useState<'all' | 'console' | 'warpgate'>('all')
  const [sorting, setSorting] = useState<SortingState>([
    { id: 'timestamp', desc: true },
  ])
  const [globalFilter, setGlobalFilter] = useState('')

  const q = useQuery({
    queryKey: ['unifiedAudit', source],
    queryFn: () =>
      listUnifiedAuditFn({ data: { limit: 250, source } }),
    staleTime: 10_000,
    refetchInterval: 30_000,
  })

  const entries = q.data?.events || []

  const columns: ColumnDef<TimelineEvent>[] = useMemo(
    () => [
      {
        accessorKey: 'timestamp',
        header: t('audit.columns.timestamp'),
        cell: ({ row }) => {
          const d = new Date(row.original.timestamp)
          return (
            <span className="text-sm tabular-nums">
              {d.toLocaleDateString(i18n.language === 'en' ? 'en-US' : 'pt-BR')}{' '}
              {d.toLocaleTimeString(i18n.language === 'en' ? 'en-US' : 'pt-BR')}
            </span>
          )
        },
        size: 160,
      },
      {
        accessorKey: 'source',
        header: 'Fonte',
        cell: ({ row }) => (
          <Badge
            variant={
              row.original.source === 'console'
                ? 'default'
                : row.original.source === 'warpgate'
                  ? 'secondary'
                  : 'outline'
            }
            className="text-xs"
          >
            {row.original.source}
          </Badge>
        ),
        size: 90,
      },
      {
        accessorKey: 'action',
        header: t('audit.columns.action'),
        cell: ({ row }) => (
          <span className="text-sm font-medium">{row.original.action}</span>
        ),
      },
      {
        accessorKey: 'actor',
        header: t('audit.columns.actor'),
        cell: ({ row }) => (
          <span className="font-mono text-sm">{row.original.actor}</span>
        ),
      },
      {
        accessorKey: 'target',
        header: t('audit.columns.target'),
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground font-mono text-xs">
            {row.original.target ?? '—'}
          </span>
        ),
      },
      {
        accessorKey: 'result',
        header: t('audit.columns.result'),
        cell: ({ row }) => {
          const r = row.original.result
          if (r === 'success') {
            return (
              <span className="inline-flex items-center gap-1 text-emerald-600 text-sm">
                <CheckCircle2 className="h-3.5 w-3.5" /> ok
              </span>
            )
          }
          if (r === 'error') {
            return (
              <span className="inline-flex items-center gap-1 text-destructive text-sm">
                <XCircle className="h-3.5 w-3.5" /> error
              </span>
            )
          }
          return (
            <span className="inline-flex items-center gap-1 text-muted-foreground text-sm">
              <Info className="h-3.5 w-3.5" /> info
            </span>
          )
        },
        size: 90,
      },
      {
        accessorKey: 'detail',
        header: 'Detalhe',
        cell: ({ row }) => (
          <span className="text-xs text-muted-foreground line-clamp-2 max-w-[240px]">
            {row.original.detail || '—'}
          </span>
        ),
      },
    ],
    [t, i18n.language],
  )

  const table = useReactTable({
    data: entries,
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

  if (q.isLoading) {
    return (
      <div className="p-6 space-y-3">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <ClipboardList className="h-7 w-7 text-primary" />
            {t('audit.title', { defaultValue: 'Auditoria' })}
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            Timeline unificada: ações do console + sessões Warpgate (quando a
            API expõe).
            {q.data?.sources && (
              <span className="ml-2 font-mono text-xs">
                console={q.data.sources.console} · warpgate=
                {q.data.sources.warpgate}
              </span>
            )}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Select
            value={source}
            onValueChange={(v) => setSource(v as typeof source)}
          >
            <SelectTrigger className="w-[140px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Todas fontes</SelectItem>
              <SelectItem value="console">Console</SelectItem>
              <SelectItem value="warpgate">Warpgate</SelectItem>
            </SelectContent>
          </Select>
          <div className="relative">
            <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              className="pl-8 w-56"
              placeholder="Filtrar…"
              value={globalFilter}
              onChange={(e) => setGlobalFilter(e.target.value)}
            />
          </div>
          <Button
            variant="outline"
            size="icon"
            onClick={() => void q.refetch()}
            disabled={q.isFetching}
          >
            <RefreshCw
              className={`h-4 w-4 ${q.isFetching ? 'animate-spin' : ''}`}
            />
          </Button>
        </div>
      </div>

      {entries.length === 0 ? (
        <EmptyState
          title="Sem eventos"
          description="Ainda não há activity log ou sessões visíveis."
        />
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((hg) => (
                <TableRow key={hg.id}>
                  {hg.headers.map((h) => (
                    <TableHead key={h.id}>
                      {h.isPlaceholder
                        ? null
                        : flexRender(
                            h.column.columnDef.header,
                            h.getContext(),
                          )}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {table.getRowModel().rows.map((row) => (
                <TableRow key={row.id}>
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
      )}
    </div>
  )
}
