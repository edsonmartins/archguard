// src/components/identity/person-list-page.tsx

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
  type ColumnFiltersState,
} from '@tanstack/react-table'
import { UserPlus, Upload, Search, MoreHorizontal, Trash2, KeySquare, Eye } from 'lucide-react'
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Checkbox } from '@/components/ui/checkbox'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge } from '@/components/shared/status-badge'
import { EmptyState } from '@/components/shared/empty-state'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { PermissionGate } from '@/components/shared/permission-gate'
import { BulkActionsToolbar } from '@/components/identity/bulk-actions-toolbar'
import { usePersons, useDeletePerson } from '@/lib/hooks/use-persons'
import { useTenantFilter } from '@/lib/hooks/use-tenant-filter'
import { initials } from '@/lib/utils/formatters'
import type { Person } from '@/lib/api/types/kanidm'

export function PersonListPage() {
  const { data: persons, isLoading } = usePersons()
  const deletePerson = useDeletePerson()
  const { filterPersons } = useTenantFilter()

  const [sorting, setSorting] = useState<SortingState>([])
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
  const [globalFilter, setGlobalFilter] = useState('')
  const [rowSelection, setRowSelection] = useState({})
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [deleteTarget, setDeleteTarget] = useState<Person | null>(null)

  const filteredData = useMemo(() => {
    if (!persons) return []
    const tenantFiltered = filterPersons(persons)
    if (statusFilter === 'all') return tenantFiltered
    return tenantFiltered.filter((p) => p.status === statusFilter)
  }, [persons, statusFilter, filterPersons])

  const columns: ColumnDef<Person>[] = useMemo(
    () => [
      {
        id: 'select',
        header: ({ table }) => (
          <Checkbox
            checked={
              table.getIsAllPageRowsSelected() ||
              (table.getIsSomePageRowsSelected() && 'indeterminate')
            }
            onCheckedChange={(value) =>
              table.toggleAllPageRowsSelected(!!value)
            }
            aria-label="Selecionar todos"
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            checked={row.getIsSelected()}
            onCheckedChange={(value) => row.toggleSelected(!!value)}
            aria-label="Selecionar linha"
          />
        ),
        enableSorting: false,
        size: 40,
      },
      {
        accessorKey: 'displayName',
        header: 'Nome',
        cell: ({ row }) => {
          const person = row.original
          return (
            <Link
              to="/identities/$personId"
              params={{ personId: person.id }}
              className="flex items-center gap-3 hover:underline"
            >
              <Avatar className="h-8 w-8">
                <AvatarFallback className="text-xs">
                  {initials(person.displayName)}
                </AvatarFallback>
              </Avatar>
              <div>
                <p className="font-medium">{person.displayName}</p>
                <p className="text-xs text-muted-foreground">
                  @{person.username}
                </p>
              </div>
            </Link>
          )
        },
      },
      {
        accessorKey: 'emails',
        header: 'Email',
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground">
            {row.original.emails[0] ?? '—'}
          </span>
        ),
      },
      {
        accessorKey: 'groupNames',
        header: 'Grupos',
        cell: ({ row }) => {
          const groups = row.original.groupNames.slice(0, 3)
          const remaining = row.original.groupNames.length - 3
          return (
            <div className="flex flex-wrap gap-1">
              {groups.map((g) => (
                <Badge key={g} variant="outline" className="text-xs">
                  {g}
                </Badge>
              ))}
              {remaining > 0 && (
                <Badge variant="secondary" className="text-xs">
                  +{remaining}
                </Badge>
              )}
            </div>
          )
        },
        enableSorting: false,
      },
      {
        accessorKey: 'credentialStatus',
        header: 'MFA',
        cell: ({ row }) => {
          const cred = row.original.credentialStatus
          if (!cred) return <span className="text-xs text-muted-foreground">—</span>
          if (cred.hasPasskeys || cred.hasWebauthn) {
            return <Badge variant="default" className="text-xs">Passkey</Badge>
          }
          if (cred.hasTotp) {
            return <Badge variant="secondary" className="text-xs">TOTP</Badge>
          }
          if (cred.hasPassword) {
            return <Badge variant="outline" className="text-xs">Senha</Badge>
          }
          return <Badge variant="destructive" className="text-xs">Nenhum</Badge>
        },
        enableSorting: false,
        size: 80,
      },
      {
        accessorKey: 'status',
        header: 'Status',
        cell: ({ row }) => <StatusBadge status={row.original.status} />,
        size: 100,
      },
      {
        id: 'actions',
        cell: ({ row }) => {
          const person = row.original
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
                    to="/identities/$personId"
                    params={{ personId: person.id }}
                  >
                    <Eye className="mr-2 h-4 w-4" />
                    Ver Detalhes
                  </Link>
                </DropdownMenuItem>
                <PermissionGate require="persons:credentials">
                  <DropdownMenuItem asChild>
                    <Link
                      to="/identities/$personId"
                      params={{ personId: person.id }}
                      search={{ tab: 'credentials' }}
                    >
                      <KeySquare className="mr-2 h-4 w-4" />
                      Credenciais
                    </Link>
                  </DropdownMenuItem>
                </PermissionGate>
                <PermissionGate require="persons:delete">
                  <DropdownMenuItem
                    className="text-destructive"
                    onClick={() => setDeleteTarget(person)}
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
    data: filteredData,
    columns,
    state: {
      sorting,
      columnFilters,
      globalFilter,
      rowSelection,
    },
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    onGlobalFilterChange: setGlobalFilter,
    onRowSelectionChange: setRowSelection,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: {
      pagination: { pageSize: 25 },
    },
  })

  if (isLoading) {
    return <PersonListSkeleton />
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Identidades</h1>
          <p className="text-muted-foreground">
            Gerencie pessoas e suas credenciais
          </p>
        </div>
        <div className="flex gap-2">
          <PermissionGate require="persons:create">
            <Button asChild variant="outline">
              <Link to="/identities/import" as={undefined}>
                <Upload className="mr-2 h-4 w-4" />
                Importar CSV
              </Link>
            </Button>
            <Button asChild>
              <Link to="/identities/create" as={undefined}>
                <UserPlus className="mr-2 h-4 w-4" />
                Nova Pessoa
              </Link>
            </Button>
          </PermissionGate>
        </div>
      </div>

      <div className="flex items-center gap-4">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Buscar por nome, email ou username..."
            value={globalFilter}
            onChange={(e) => setGlobalFilter(e.target.value)}
            className="pl-10"
          />
        </div>
        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger className="w-[150px]">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Todos</SelectItem>
            <SelectItem value="active">Ativos</SelectItem>
            <SelectItem value="locked">Bloqueados</SelectItem>
            <SelectItem value="expired">Expirados</SelectItem>
            <SelectItem value="disabled">Desabilitados</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Bulk Actions Toolbar */}
      {Object.keys(rowSelection).length > 0 && (
        <BulkActionsToolbar
          selectedPersons={table
            .getFilteredSelectedRowModel()
            .rows.map((r) => r.original)}
          onClearSelection={() => setRowSelection({})}
        />
      )}

      {filteredData.length === 0 ? (
        <EmptyState
          title="Nenhuma pessoa encontrada"
          description="Crie uma nova pessoa ou ajuste os filtros de busca."
          action={{ label: 'Nova Pessoa', to: '/identities/create' }}
        />
      ) : (
        <>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                {table.getHeaderGroups().map((headerGroup) => (
                  <TableRow key={headerGroup.id}>
                    {headerGroup.headers.map((header) => (
                      <TableHead
                        key={header.id}
                        style={{ width: header.getSize() }}
                      >
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
                    data-state={row.getIsSelected() && 'selected'}
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

          <div className="flex items-center justify-between">
            <p className="text-sm text-muted-foreground">
              {table.getFilteredSelectedRowModel().rows.length} de{' '}
              {table.getFilteredRowModel().rows.length} selecionado(s)
            </p>
            <div className="flex items-center gap-2">
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
          </div>
        </>
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Excluir Pessoa"
        description={`Esta ação é irreversível. Todos os dados de ${deleteTarget?.displayName} serão permanentemente removidos.`}
        confirmText={deleteTarget?.username ?? ''}
        destructive
        isLoading={deletePerson.isPending}
        onConfirm={() => {
          if (deleteTarget) {
            deletePerson.mutate(deleteTarget.id, {
              onSuccess: () => setDeleteTarget(null),
            })
          }
        }}
      />
    </div>
  )
}

function PersonListSkeleton() {
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <Skeleton className="h-9 w-48" />
          <Skeleton className="mt-2 h-5 w-72" />
        </div>
        <div className="flex gap-2">
          <Skeleton className="h-10 w-32" />
          <Skeleton className="h-10 w-32" />
        </div>
      </div>
      <div className="flex items-center gap-4">
        <Skeleton className="h-10 w-80" />
        <Skeleton className="h-10 w-36" />
      </div>
      <div className="rounded-md border">
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} className="flex items-center gap-4 border-b p-4">
            <Skeleton className="h-4 w-4" />
            <Skeleton className="h-8 w-8 rounded-full" />
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-4 w-48" />
            <Skeleton className="h-5 w-16" />
            <Skeleton className="h-5 w-16" />
          </div>
        ))}
      </div>
    </div>
  )
}
