import { useTranslation } from 'react-i18next'
// src/components/group/group-list-page.tsx

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
  UsersRound,
  Search,
  MoreHorizontal,
  Trash2,
  Eye,
  List,
  Network,
  Lock,
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
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { EmptyState } from '@/components/shared/empty-state'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { PermissionGate } from '@/components/shared/permission-gate'
import { GroupBadge } from '@/components/shared/group-badge'
import { useGroups, useDeleteGroup } from '@/lib/hooks/use-groups'
import { useTenantFilter } from '@/lib/hooks/use-tenant-filter'
import type { Group } from '@/lib/api/types/kanidm'

type ViewMode = 'list' | 'tree'

export function GroupListPage() {
  const { t } = useTranslation()
  const { data: groups, isLoading } = useGroups()
  const deleteGroup = useDeleteGroup()
  const { filterGroups, isFiltering } = useTenantFilter()

  const [sorting, setSorting] = useState<SortingState>([])
  const [globalFilter, setGlobalFilter] = useState('')
  const [viewMode, setViewMode] = useState<ViewMode>('list')
  const [deleteTarget, setDeleteTarget] = useState<Group | null>(null)

  const filteredData = useMemo(() => {
    if (!groups) return []
    // When tenant filter is active, show only tenant groups; otherwise show all
    return isFiltering ? filterGroups(groups) : groups
  }, [groups, filterGroups, isFiltering])

  const columns: ColumnDef<Group>[] = useMemo(
    () => [
      {
        accessorKey: 'name',
        header: 'Nome',
        cell: ({ row }) => {
          const group = row.original
          return (
            <Link
              to="/groups/$groupId"
              params={{ groupId: group.id }}
              className="flex items-center gap-2 hover:underline"
            >
              <span className="font-medium">{group.name}</span>
              {group.isBuiltin && (
                <Tooltip>
                  <TooltipTrigger aria-label={t('groupsPage.systemGroup')}>
                    <Lock className="h-3 w-3 text-muted-foreground" />
                  </TooltipTrigger>
                  <TooltipContent>{t('groupsPage.systemGroupHint')}</TooltipContent>
                </Tooltip>
              )}
            </Link>
          )
        },
      },
      {
        accessorKey: 'description',
        header: t('groupsPage.columns.description'),
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground">
            {row.original.description ?? '—'}
          </span>
        ),
      },
      {
        accessorKey: 'memberCount',
        header: 'Membros',
        cell: ({ row }) => (
          <Badge variant="outline">{row.original.memberCount}</Badge>
        ),
        size: 100,
      },
      {
        id: 'type',
        header: 'Tipo',
        cell: ({ row }) => <GroupBadge name={row.original.name} />,
        size: 120,
      },
      {
        id: 'actions',
        cell: ({ row }) => {
          const group = row.original
          return (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  aria-label="Ações"
                >
                  <MoreHorizontal className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem asChild>
                  <Link
                    to="/groups/$groupId"
                    params={{ groupId: group.id }}
                  >
                    <Eye className="mr-2 h-4 w-4" />
                    Ver Detalhes
                  </Link>
                </DropdownMenuItem>
                {!group.isBuiltin && (
                  <PermissionGate require="groups:delete">
                    <DropdownMenuItem
                      className="text-destructive"
                      onClick={() => setDeleteTarget(group)}
                    >
                      <Trash2 className="mr-2 h-4 w-4" />
                      Excluir
                    </DropdownMenuItem>
                  </PermissionGate>
                )}
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
    return <GroupListSkeleton />
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Grupos</h1>
          <p className="text-muted-foreground">
            Gerencie grupos e associações de membros
          </p>
        </div>
        <PermissionGate require="groups:create">
          <Button asChild>
            <Link to="/groups/create">
              <UsersRound className="mr-2 h-4 w-4" />
              Novo Grupo
            </Link>
          </Button>
        </PermissionGate>
      </div>

      <div className="flex items-center gap-4">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder={t('groupsPage.search')}
            value={globalFilter}
            onChange={(e) => setGlobalFilter(e.target.value)}
            className="pl-10"
          />
        </div>
        <div className="flex rounded-md border">
          <Button
            variant={viewMode === 'list' ? 'secondary' : 'ghost'}
            size="icon"
            className="h-9 w-9 rounded-r-none"
            onClick={() => setViewMode('list')}
            aria-label={t('groupsPage.listView')}
          >
            <List className="h-4 w-4" />
          </Button>
          <Button
            variant={viewMode === 'tree' ? 'secondary' : 'ghost'}
            size="icon"
            className="h-9 w-9 rounded-l-none"
            onClick={() => setViewMode('tree')}
            aria-label={t('groupsPage.treeView')}
          >
            <Network className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {filteredData.length === 0 ? (
        <EmptyState
          icon={UsersRound}
          title={t('groupsPage.empty')}
          description={t('groupsPage.emptyHint')}
          action={{ label: t('groupsPage.create'), to: '/groups/create' }}
        />
      ) : viewMode === 'list' ? (
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
      ) : (
        <GroupTreeView groups={filteredData} />
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t('groupsPage.deleteTitle')}
        description={`Esta ação é irreversível. O grupo "${deleteTarget?.name}" será permanentemente removido.`}
        confirmText={deleteTarget?.name ?? ''}
        destructive
        isLoading={deleteGroup.isPending}
        onConfirm={() => {
          if (deleteTarget) {
            deleteGroup.mutate(deleteTarget.id, {
              onSuccess: () => setDeleteTarget(null),
            })
          }
        }}
      />
    </div>
  )
}

function GroupTreeView({ groups }: { groups: Group[] }) {
  const builtinGroups = groups.filter((g) => g.isBuiltin)
  const customGroups = groups.filter((g) => !g.isBuiltin)

  return (
    <div className="space-y-4">
      {builtinGroups.length > 0 && (
        <div>
          <h3 className="mb-2 text-sm font-medium text-muted-foreground">
            Grupos do Sistema
          </h3>
          <div className="space-y-1">
            {builtinGroups.map((group) => (
              <GroupTreeItem key={group.id} group={group} />
            ))}
          </div>
        </div>
      )}
      {customGroups.length > 0 && (
        <div>
          <h3 className="mb-2 text-sm font-medium text-muted-foreground">
            Grupos Personalizados
          </h3>
          <div className="space-y-1">
            {customGroups.map((group) => (
              <GroupTreeItem key={group.id} group={group} />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function GroupTreeItem({ group }: { group: Group }) {
  return (
    <Link
      to="/groups/$groupId"
      params={{ groupId: group.id }}
      className="flex items-center justify-between rounded-lg border p-3 hover:bg-accent"
    >
      <div className="flex items-center gap-3">
        <UsersRound className="h-4 w-4 text-muted-foreground" />
        <div>
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium">{group.name}</span>
            {group.isBuiltin && (
              <Lock className="h-3 w-3 text-muted-foreground" />
            )}
          </div>
          {group.description && (
            <p className="text-xs text-muted-foreground">
              {group.description}
            </p>
          )}
        </div>
      </div>
      <Badge variant="outline">{group.memberCount} membros</Badge>
    </Link>
  )
}

function GroupListSkeleton() {
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <Skeleton className="h-9 w-36" />
          <Skeleton className="mt-2 h-5 w-64" />
        </div>
        <Skeleton className="h-10 w-32" />
      </div>
      <Skeleton className="h-10 w-80" />
      <div className="rounded-md border">
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} className="flex items-center gap-4 border-b p-4">
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-4 w-64" />
            <Skeleton className="h-5 w-16" />
          </div>
        ))}
      </div>
    </div>
  )
}
