import { useTranslation } from 'react-i18next'
// src/components/audit/audit-page.tsx
// Repurposed: Activity Log (console-side) instead of Kanidm audit (not available)

import { useState, useMemo } from 'react'
import { Link } from '@tanstack/react-router'
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
  CheckCircle2,
  XCircle,
  Trash2,
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
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/shared/empty-state'
import { useActivityLog } from '@/lib/hooks/use-activity-log'
import type { ActivityLogEntry } from '@/lib/api/types/kanidm'

export function AuditPage() {
  const { t, i18n } = useTranslation()
  const { data: entries, isLoading } = useActivityLog()
  const [sorting, setSorting] = useState<SortingState>([
    { id: 'timestamp', desc: true },
  ])
  const [globalFilter, setGlobalFilter] = useState('')

  const columns: ColumnDef<ActivityLogEntry>[] = useMemo(
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
          <span className="text-sm text-muted-foreground">
            {row.original.target ?? '—'}
          </span>
        ),
      },
      {
        accessorKey: 'method',
        header: 'Método',
        cell: ({ row }) => (
          <Badge variant="outline" className="text-xs font-mono">
            {row.original.method}
          </Badge>
        ),
        size: 80,
      },
      {
        accessorKey: 'result',
        header: t('audit.columns.result'),
        cell: ({ row }) => {
          const ok = row.original.result === 'success'
          return (
            <div className="flex items-center gap-1">
              {ok ? (
                <CheckCircle2 className="h-4 w-4 text-green-500" />
              ) : (
                <XCircle className="h-4 w-4 text-destructive" />
              )}
              <span className="text-xs">{ok ? 'OK' : 'Erro'}</span>
            </div>
          )
        },
        size: 80,
      },
    ],
    [],
  )

  const table = useReactTable({
    data: entries ?? [],
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
    if (!entries?.length) return
    let content: string
    let type: string
    let ext: string

    if (format === 'json') {
      content = JSON.stringify(entries, null, 2)
      type = 'application/json'
      ext = 'json'
    } else {
      const headers = 'timestamp,action,actor,target,method,path,result'
      const rows = entries.map(
        (e) =>
          `${e.timestamp},${e.action},${e.actor},${e.target ?? ''},${e.method},${e.path},${e.result}`,
      )
      content = [headers, ...rows].join('\n')
      type = 'text/csv'
      ext = 'csv'
    }

    const blob = new Blob([content], { type })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `activity-log-${new Date().toISOString().slice(0, 10)}.${ext}`
    a.click()
    URL.revokeObjectURL(url)
  }

  if (isLoading) return <AuditSkeleton />

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">{t('audit.title')}</h1>
          <p className="text-muted-foreground">
            Registro de operações realizadas pelo console
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" asChild>
            <Link to="/recycle-bin">
              <Trash2 className="mr-2 h-4 w-4" />
              Lixeira
            </Link>
          </Button>
          <Button variant="outline" size="sm" onClick={() => handleExport('csv')}>
            <Download className="mr-2 h-4 w-4" />
            CSV
          </Button>
          <Button variant="outline" size="sm" onClick={() => handleExport('json')}>
            <Download className="mr-2 h-4 w-4" />
            JSON
          </Button>
        </div>
      </div>

      <div className="rounded-lg border border-dashed p-3 text-sm text-muted-foreground">
        <Info className="mr-2 inline h-4 w-4" />
        O Kanidm v1.9 não possui API de auditoria pública. Este log registra apenas
        operações de escrita (criação, edição, exclusão) realizadas através do Console.
      </div>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder={t('audit.search')}
          value={globalFilter}
          onChange={(e) => setGlobalFilter(e.target.value)}
          className="pl-10"
        />
      </div>

      {!entries || entries.length === 0 ? (
        <EmptyState
          icon={ClipboardList}
          title={t('audit.empty')}
          description={t('audit.emptyHint')}
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
                          : flexRender(header.column.columnDef.header, header.getContext())}
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
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
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
    </div>
  )
}

function AuditSkeleton() {
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <Skeleton className="h-9 w-48" />
          <Skeleton className="mt-2 h-5 w-72" />
        </div>
      </div>
      <Skeleton className="h-10 w-80" />
      <div className="rounded-md border">
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} className="flex items-center gap-4 border-b p-4">
            <Skeleton className="h-4 w-28" />
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-4 w-16" />
          </div>
        ))}
      </div>
    </div>
  )
}
