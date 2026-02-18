// src/components/service-account/sa-list-page.tsx

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
  Bot,
  Search,
  MoreHorizontal,
  Trash2,
  Eye,
  Plus,
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge } from '@/components/shared/status-badge'
import { EmptyState } from '@/components/shared/empty-state'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { PermissionGate } from '@/components/shared/permission-gate'
import {
  useServiceAccounts,
  useDeleteServiceAccount,
} from '@/lib/hooks/use-service-accounts'
import { useTenantFilter } from '@/lib/hooks/use-tenant-filter'
import type { ServiceAccount } from '@/lib/api/types/kanidm'

export function ServiceAccountListPage() {
  const { data: accounts, isLoading } = useServiceAccounts()
  const deleteAccount = useDeleteServiceAccount()
  const { filterServiceAccounts } = useTenantFilter()

  const filteredAccounts = useMemo(
    () => filterServiceAccounts(accounts ?? []),
    [accounts, filterServiceAccounts],
  )

  const [sorting, setSorting] = useState<SortingState>([])
  const [globalFilter, setGlobalFilter] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<ServiceAccount | null>(null)

  const columns: ColumnDef<ServiceAccount>[] = useMemo(
    () => [
      {
        accessorKey: 'displayName',
        header: 'Nome',
        cell: ({ row }) => {
          const sa = row.original
          return (
            <Link
              to="/service-accounts/$accountId"
              params={{ accountId: sa.id }}
              className="flex items-center gap-2 hover:underline"
            >
              <Bot className="h-4 w-4 text-muted-foreground" />
              <div>
                <p className="font-medium">{sa.displayName}</p>
                <p className="text-xs text-muted-foreground">@{sa.name}</p>
              </div>
            </Link>
          )
        },
      },
      {
        accessorKey: 'description',
        header: 'Descrição',
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground">
            {row.original.description ?? '—'}
          </span>
        ),
      },
      {
        accessorKey: 'apiTokens',
        header: 'Tokens',
        cell: ({ row }) => (
          <Badge variant="outline">
            {row.original.apiTokens.length} token(s)
          </Badge>
        ),
        size: 100,
      },
      {
        accessorKey: 'groupNames',
        header: 'Grupos',
        cell: ({ row }) => (
          <div className="flex flex-wrap gap-1">
            {row.original.groupNames.slice(0, 3).map((g) => (
              <Badge key={g} variant="outline" className="text-xs">
                {g}
              </Badge>
            ))}
            {row.original.groupNames.length > 3 && (
              <Badge variant="secondary" className="text-xs">
                +{row.original.groupNames.length - 3}
              </Badge>
            )}
          </div>
        ),
        enableSorting: false,
      },
      {
        accessorKey: 'status',
        header: 'Status',
        cell: ({ row }) => (
          <StatusBadge status={row.original.status as 'active' | 'expired' | 'disabled'} />
        ),
        size: 100,
      },
      {
        id: 'actions',
        cell: ({ row }) => {
          const sa = row.original
          return (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" className="h-8 w-8">
                  <MoreHorizontal className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem asChild>
                  <Link
                    to="/service-accounts/$accountId"
                    params={{ accountId: sa.id }}
                  >
                    <Eye className="mr-2 h-4 w-4" />
                    Ver Detalhes
                  </Link>
                </DropdownMenuItem>
                <PermissionGate require="service_accounts:delete">
                  <DropdownMenuItem
                    className="text-destructive"
                    onClick={() => setDeleteTarget(sa)}
                  >
                    <Trash2 className="mr-2 h-4 w-4" />
                    Excluir
                  </DropdownMenuItem>
                </PermissionGate>
              </DropdownMenuContent>
            </DropdownMenu>
          )
        },
        size: 50,
      },
    ],
    [],
  )

  const table = useReactTable({
    data: filteredAccounts,
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

  if (isLoading) {
    return <SAListSkeleton />
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">
            Service Accounts
          </h1>
          <p className="text-muted-foreground">
            Gerencie contas de serviço e tokens de API
          </p>
        </div>
        <PermissionGate require="service_accounts:create">
          <Button asChild>
            <Link to="/service-accounts/create" as={undefined}>
              <Plus className="mr-2 h-4 w-4" />
              Novo Service Account
            </Link>
          </Button>
        </PermissionGate>
      </div>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Buscar service accounts..."
          value={globalFilter}
          onChange={(e) => setGlobalFilter(e.target.value)}
          className="pl-10"
        />
      </div>

      {filteredAccounts.length === 0 ? (
        <EmptyState
          icon={Bot}
          title="Nenhum service account"
          description="Crie um service account para integrações máquina-a-máquina."
          action={{ label: 'Novo Service Account', to: '/service-accounts/create' }}
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

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Excluir Service Account"
        description={`Esta ação é irreversível. O service account "${deleteTarget?.displayName}" e todos os seus tokens serão removidos.`}
        confirmText={deleteTarget?.name ?? ''}
        destructive
        isLoading={deleteAccount.isPending}
        onConfirm={() => {
          if (deleteTarget) {
            deleteAccount.mutate(deleteTarget.id, {
              onSuccess: () => setDeleteTarget(null),
            })
          }
        }}
      />
    </div>
  )
}

function SAListSkeleton() {
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <Skeleton className="h-9 w-48" />
          <Skeleton className="mt-2 h-5 w-72" />
        </div>
        <Skeleton className="h-10 w-44" />
      </div>
      <Skeleton className="h-10 w-80" />
      <div className="rounded-md border">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex items-center gap-4 border-b p-4">
            <Skeleton className="h-4 w-4" />
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-4 w-48" />
            <Skeleton className="h-5 w-16" />
          </div>
        ))}
      </div>
    </div>
  )
}
