// Warpgate tab — targets + roles (CP-2)

import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { enumLabel } from '@/lib/i18n/labels'
import {
  AlertTriangle,
  ExternalLink,
  Plus,
  Trash2,
} from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  createWarpgateRoleFn,
  deleteWarpgateRoleFn,
  deleteWarpgateTargetFn,
  getWarpgateStatusFn,
  listWarpgateRolesFn,
  listWarpgateTargetsFn,
  upsertWarpgateTargetFn,
} from '@/server/warpgate-fn'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'

type Props = {
  canRead: boolean
  canManage: boolean
}

export function WarpgatePanel({ canRead, canManage }: Props) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const invalidate = () => {
    void qc.invalidateQueries({ queryKey: ['warpgate'] })
  }

  const wgStatus = useQuery({
    queryKey: ['warpgate', 'status'],
    queryFn: () => getWarpgateStatusFn(),
  })
  const targets = useQuery({
    queryKey: ['warpgate', 'targets'],
    queryFn: () => listWarpgateTargetsFn(),
    enabled: canRead && !!wgStatus.data?.configured,
  })
  const roles = useQuery({
    queryKey: ['warpgate', 'roles'],
    queryFn: () => listWarpgateRolesFn(),
    enabled: canRead && !!wgStatus.data?.configured,
  })

  const [targetOpen, setTargetOpen] = useState(false)
  const [roleOpen, setRoleOpen] = useState(false)
  const [deleteTargetId, setDeleteTargetId] = useState<string | null>(null)
  const [deleteRoleId, setDeleteRoleId] = useState<string | null>(null)
  const [roleName, setRoleName] = useState('')
  const [tForm, setTForm] = useState({
    name: '',
    kind: 'Ssh' as 'Ssh' | 'Postgres' | 'MySql' | 'Http',
    host: '',
    port: 22,
    username: 'labuser',
    password: '',
    database: '',
    roles: '',
  })

  const saveTarget = useMutation({
    mutationFn: () =>
      upsertWarpgateTargetFn({
        data: {
          name: tForm.name,
          kind: tForm.kind,
          host: tForm.host,
          port: tForm.port,
          username: tForm.username || undefined,
          password: tForm.password || undefined,
          database: tForm.database || undefined,
          roles: tForm.roles
            .split(',')
            .map((r) => r.trim())
            .filter(Boolean),
        },
      }),
    onSuccess: () => {
      toast.success(t('gatewaysPage.targetSaved'))
      setTargetOpen(false)
      invalidate()
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const delTarget = useMutation({
    mutationFn: (id: string) => deleteWarpgateTargetFn({ data: { id } }),
    onSuccess: () => {
      toast.success(t('gatewaysPage.targetRemoved'))
      setDeleteTargetId(null)
      invalidate()
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const saveRole = useMutation({
    mutationFn: () =>
      createWarpgateRoleFn({ data: { name: roleName, description: '' } }),
    onSuccess: () => {
      toast.success(t('gatewaysPage.roleCreated'))
      setRoleOpen(false)
      setRoleName('')
      invalidate()
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const delRole = useMutation({
    mutationFn: (id: string) => deleteWarpgateRoleFn({ data: { id } }),
    onSuccess: () => {
      toast.success(t('gatewaysPage.roleRemoved'))
      setDeleteRoleId(null)
      invalidate()
    },
    onError: (e) => toast.error((e as Error).message),
  })

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle>{t('gatewaysPage.warpgate')}</CardTitle>
            <CardDescription>{t('gatewaysPage.warpgateDesc')}</CardDescription>
          </div>
          <div className="flex gap-2">
            {wgStatus.data?.configured && canManage && (
              <>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setRoleOpen(true)}
                >
                  <Plus className="h-4 w-4 mr-1" /> Role
                </Button>
                <Button
                  size="sm"
                  onClick={() => {
                    setTForm({
                      name: '',
                      kind: 'Ssh',
                      host: '',
                      port: 22,
                      username: 'labuser',
                      password: '',
                      database: '',
                      roles: '',
                    })
                    setTargetOpen(true)
                  }}
                >
                  <Plus className="h-4 w-4 mr-1" /> Target
                </Button>
              </>
            )}
            <Button variant="outline" size="sm" asChild>
              <a
                href={wgStatus.data?.url || 'https://wg.archgate.com.br'}
                target="_blank"
                rel="noreferrer"
              >
                UI stock <ExternalLink className="h-3.5 w-3.5 ml-1" />
              </a>
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {wgStatus.isLoading ? (
            <Skeleton className="h-6 w-48" />
          ) : (
            <div className="flex flex-wrap items-center gap-2 mb-4">
              <Badge
                variant={
                  wgStatus.data?.configured ? 'default' : 'destructive'
                }
              >
                {wgStatus.data?.configured
                  ? t('gatewaysPage.apiConfigured')
                  : t('gatewaysPage.apiNotConfigured')}
              </Badge>
              <span className="text-sm font-mono text-muted-foreground">
                {wgStatus.data?.url}
              </span>
            </div>
          )}
          {!wgStatus.data?.configured && (
            <p className="text-sm text-amber-700 dark:text-amber-400 flex gap-2">
              <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5" />
              Defina <code className="text-xs">WARPGATE_ADMIN_PASSWORD</code>{' '}
              no serviço do console.
            </p>
          )}
        </CardContent>
      </Card>

      {canRead && wgStatus.data?.configured && (
        <>
          <Card>
            <CardHeader>
              <CardTitle>Targets ({targets.data?.length ?? '…'})</CardTitle>
            </CardHeader>
            <CardContent>
              {targets.isLoading ? (
                <Skeleton className="h-24 w-full" />
              ) : targets.isError ? (
                <p className="text-sm text-destructive">
                  {(targets.error as Error).message}
                </p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Nome</TableHead>
                      <TableHead>Kind</TableHead>
                      <TableHead>Host</TableHead>
                      <TableHead>Port</TableHead>
                      <TableHead className="w-12" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(targets.data || []).map((t) => (
                      <TableRow key={t.id}>
                        <TableCell className="font-medium">{t.name}</TableCell>
                        <TableCell>{t.kind || '—'}</TableCell>
                        <TableCell className="font-mono text-xs">
                          {t.host || '—'}
                        </TableCell>
                        <TableCell>{t.port ?? '—'}</TableCell>
                        <TableCell>
                          {canManage && (
                            <Button
                              size="icon"
                              variant="ghost"
                              onClick={() => setDeleteTargetId(t.id)}
                            >
                              <Trash2 className="h-4 w-4 text-destructive" />
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

          <Card>
            <CardHeader>
              <CardTitle>Roles ({roles.data?.length ?? '…'})</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-2">
              {(roles.data || []).map((r) => (
                <Badge key={r.id} variant="outline" className="gap-1 pr-1">
                  {r.name}
                  {canManage && (
                    <button
                      type="button"
                      className="ml-1 rounded p-0.5 hover:bg-muted"
                      onClick={() => setDeleteRoleId(r.id)}
                      aria-label={`Remover role ${r.name}`}
                    >
                      <Trash2 className="h-3 w-3" />
                    </button>
                  )}
                </Badge>
              ))}
            </CardContent>
          </Card>
        </>
      )}

      <Dialog open={targetOpen} onOpenChange={setTargetOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('gatewaysPage.newTarget')}</DialogTitle>
          </DialogHeader>
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1 sm:col-span-2">
              <Label>Nome</Label>
              <Input
                value={tForm.name}
                onChange={(e) => setTForm({ ...tForm, name: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>Tipo</Label>
              <Select
                value={tForm.kind}
                onValueChange={(v) =>
                  setTForm({
                    ...tForm,
                    kind: v as typeof tForm.kind,
                    port:
                      v === 'Ssh'
                        ? 22
                        : v === 'Postgres'
                          ? 5432
                          : v === 'MySql'
                            ? 3306
                            : 443,
                  })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="Ssh">
                    {enumLabel(t, 'warpgateKind', 'Ssh')}
                  </SelectItem>
                  <SelectItem value="Postgres">
                    {enumLabel(t, 'warpgateKind', 'Postgres')}
                  </SelectItem>
                  <SelectItem value="MySql">
                    {enumLabel(t, 'warpgateKind', 'MySql')}
                  </SelectItem>
                  <SelectItem value="Http">
                    {enumLabel(t, 'warpgateKind', 'Http')}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1">
              <Label>Porta</Label>
              <Input
                type="number"
                value={tForm.port}
                onChange={(e) =>
                  setTForm({ ...tForm, port: Number(e.target.value) || 0 })
                }
              />
            </div>
            <div className="space-y-1 sm:col-span-2">
              <Label>Host</Label>
              <Input
                value={tForm.host}
                onChange={(e) => setTForm({ ...tForm, host: e.target.value })}
                className="font-mono"
              />
            </div>
            <div className="space-y-1">
              <Label>{t('gatewaysPage.targetUsername')}</Label>
              <Input
                value={tForm.username}
                onChange={(e) =>
                  setTForm({ ...tForm, username: e.target.value })
                }
              />
            </div>
            <div className="space-y-1">
              <Label>{t('gatewaysPage.targetPassword')}</Label>
              <Input
                type="password"
                value={tForm.password}
                onChange={(e) =>
                  setTForm({ ...tForm, password: e.target.value })
                }
                autoComplete="new-password"
              />
            </div>
            {tForm.kind === 'Postgres' && (
              <div className="space-y-1 sm:col-span-2">
                <Label>{t('gatewaysPage.targetDatabase')}</Label>
                <Input
                  value={tForm.database}
                  onChange={(e) =>
                    setTForm({ ...tForm, database: e.target.value })
                  }
                />
              </div>
            )}
            <div className="space-y-1 sm:col-span-2">
              <Label>{t('gatewaysPage.targetRoles')}</Label>
              <Input
                value={tForm.roles}
                onChange={(e) => setTForm({ ...tForm, roles: e.target.value })}
                placeholder="tenant-rio-quality"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setTargetOpen(false)}>
              Cancelar
            </Button>
            <Button
              onClick={() => saveTarget.mutate()}
              disabled={saveTarget.isPending || !tForm.name || !tForm.host}
            >
              Salvar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={roleOpen} onOpenChange={setRoleOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('gatewaysPage.newRole')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-2">
            <Label>Nome</Label>
            <Input
              value={roleName}
              onChange={(e) => setRoleName(e.target.value)}
              placeholder="tenant-acme"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRoleOpen(false)}>
              Cancelar
            </Button>
            <Button
              onClick={() => saveRole.mutate()}
              disabled={!roleName || saveRole.isPending}
            >
              Criar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!deleteTargetId}
        onOpenChange={(o) => !o && setDeleteTargetId(null)}
        title={t('gatewaysPage.deleteTarget')}
        description={t('gatewaysPage.deleteTargetDesc')}
        confirmText="REMOVER"
        destructive
        isLoading={delTarget.isPending}
        onConfirm={() => {
          if (deleteTargetId) delTarget.mutate(deleteTargetId)
        }}
      />
      <ConfirmDialog
        open={!!deleteRoleId}
        onOpenChange={(o) => !o && setDeleteRoleId(null)}
        title={t('gatewaysPage.deleteRole')}
        description={t('gatewaysPage.deleteRoleDesc')}
        confirmText="REMOVER"
        destructive
        isLoading={delRole.isPending}
        onConfirm={() => {
          if (deleteRoleId) delRole.mutate(deleteRoleId)
        }}
      />
    </div>
  )
}
