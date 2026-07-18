// Guacamole tab — browser connections (CP-3)

import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { enumLabel } from '@/lib/i18n/labels'
import { ExternalLink, Monitor, Plus, Trash2 } from 'lucide-react'
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
  createGuacamoleConnectionFn,
  deleteGuacamoleConnectionFn,
  getGuacamoleStatusFn,
  listGuacamoleConnectionsFn,
} from '@/server/guacamole-fn'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'

type Props = {
  canRead: boolean
  canManage: boolean
}

export function GuacamolePanel({ canRead, canManage }: Props) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const invalidate = () => {
    void qc.invalidateQueries({ queryKey: ['guacamole'] })
  }

  const guacStatus = useQuery({
    queryKey: ['guacamole', 'status'],
    queryFn: () => getGuacamoleStatusFn(),
  })
  const guacConns = useQuery({
    queryKey: ['guacamole', 'connections'],
    queryFn: () => listGuacamoleConnectionsFn(),
    enabled: canRead,
    retry: 1,
  })

  const [guacOpen, setGuacOpen] = useState(false)
  const [deleteGuacId, setDeleteGuacId] = useState<string | null>(null)
  const [gForm, setGForm] = useState({
    name: '',
    protocol: 'ssh' as 'ssh' | 'rdp' | 'vnc',
    hostname: '',
    port: 22,
    username: 'labuser',
    password: '',
  })

  const saveGuac = useMutation({
    mutationFn: () =>
      createGuacamoleConnectionFn({
        data: {
          name: gForm.name,
          protocol: gForm.protocol,
          hostname: gForm.hostname,
          port: gForm.port,
          username: gForm.username || undefined,
          password: gForm.password || undefined,
        },
      }),
    onSuccess: () => {
      toast.success(t('gatewaysPage.connCreated'))
      setGuacOpen(false)
      invalidate()
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const delGuac = useMutation({
    mutationFn: (id: string) => deleteGuacamoleConnectionFn({ data: { id } }),
    onSuccess: () => {
      toast.success(t('gatewaysPage.connRemoved'))
      setDeleteGuacId(null)
      invalidate()
    },
    onError: (e) => toast.error((e as Error).message),
  })

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle className="flex items-center gap-2">
              <Monitor className="h-5 w-5" />
              Guacamole
            </CardTitle>
            <CardDescription>
              Conexões browser (SSH / RDP / VNC)
            </CardDescription>
          </div>
          <div className="flex gap-2">
            {canManage && (
              <Button
                size="sm"
                onClick={() => {
                  setGForm({
                    name: '',
                    protocol: 'ssh',
                    hostname: '',
                    port: 22,
                    username: 'labuser',
                    password: '',
                  })
                  setGuacOpen(true)
                }}
              >
                <Plus className="h-4 w-4 mr-1" /> Conexão
              </Button>
            )}
            <Button variant="outline" size="sm" asChild>
              <a
                href={guacStatus.data?.url || 'https://guac.archgate.com.br'}
                target="_blank"
                rel="noreferrer"
              >
                Abrir app <ExternalLink className="h-3.5 w-3.5 ml-1" />
              </a>
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {guacConns.isLoading ? (
            <Skeleton className="h-24 w-full" />
          ) : guacConns.isError ? (
            <p className="text-sm text-destructive">
              {(guacConns.error as Error).message}
              <span className="block text-muted-foreground mt-1">
                Verifique <code className="text-xs">GUACAMOLE_URL</code> e{' '}
                <code className="text-xs">GUACAMOLE_ADMIN_PASSWORD</code> no
                console.
              </span>
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Nome</TableHead>
                  <TableHead>{t('gatewaysPage.protocol')}</TableHead>
                  <TableHead>ID</TableHead>
                  <TableHead className="w-12" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {(guacConns.data || []).map((c) => (
                  <TableRow key={c.id}>
                    <TableCell className="font-medium">{c.name}</TableCell>
                    <TableCell>
                      <Badge variant="secondary">{c.protocol}</Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs">{c.id}</TableCell>
                    <TableCell>
                      {canManage && (
                        <Button
                          size="icon"
                          variant="ghost"
                          onClick={() => setDeleteGuacId(c.id)}
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

      <Dialog open={guacOpen} onOpenChange={setGuacOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('gatewaysPage.newConnection')}</DialogTitle>
          </DialogHeader>
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1 sm:col-span-2">
              <Label>Nome</Label>
              <Input
                value={gForm.name}
                onChange={(e) => setGForm({ ...gForm, name: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>{t('gatewaysPage.protocol')}</Label>
              <Select
                value={gForm.protocol}
                onValueChange={(v) =>
                  setGForm({
                    ...gForm,
                    protocol: v as typeof gForm.protocol,
                    port: v === 'rdp' ? 3389 : v === 'vnc' ? 5900 : 22,
                  })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="ssh">
                    {enumLabel(t, 'protocol', 'ssh')}
                  </SelectItem>
                  <SelectItem value="rdp">
                    {enumLabel(t, 'protocol', 'rdp')}
                  </SelectItem>
                  <SelectItem value="vnc">
                    {enumLabel(t, 'protocol', 'vnc')}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1">
              <Label>Porta</Label>
              <Input
                type="number"
                value={gForm.port}
                onChange={(e) =>
                  setGForm({ ...gForm, port: Number(e.target.value) || 0 })
                }
              />
            </div>
            <div className="space-y-1 sm:col-span-2">
              <Label>{t('gatewaysPage.hostname')}</Label>
              <Input
                value={gForm.hostname}
                onChange={(e) =>
                  setGForm({ ...gForm, hostname: e.target.value })
                }
                className="font-mono"
              />
            </div>
            <div className="space-y-1">
              <Label>{t('common.username')}</Label>
              <Input
                value={gForm.username}
                onChange={(e) =>
                  setGForm({ ...gForm, username: e.target.value })
                }
              />
            </div>
            <div className="space-y-1">
              <Label>{t('gatewaysPage.targetPassword')}</Label>
              <Input
                type="password"
                value={gForm.password}
                onChange={(e) =>
                  setGForm({ ...gForm, password: e.target.value })
                }
                autoComplete="new-password"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setGuacOpen(false)}>
              Cancelar
            </Button>
            <Button
              onClick={() => saveGuac.mutate()}
              disabled={saveGuac.isPending || !gForm.name || !gForm.hostname}
            >
              Criar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!deleteGuacId}
        onOpenChange={(o) => !o && setDeleteGuacId(null)}
        title={t('gatewaysPage.deleteConn')}
        description={t('gatewaysPage.deleteConnDesc')}
        confirmText="REMOVER"
        destructive
        isLoading={delGuac.isPending}
        onConfirm={() => {
          if (deleteGuacId) delGuac.mutate(deleteGuacId)
        }}
      />
    </div>
  )
}
