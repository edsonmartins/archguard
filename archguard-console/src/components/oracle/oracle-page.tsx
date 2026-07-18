// W-C4 — Oracle dynamic credentials (oracle-proxy + optional OpenBao role)

import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Database,
  RefreshCw,
  KeyRound,
  Copy,
  Trash2,
  AlertTriangle,
  CheckCircle2,
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Skeleton } from '@/components/ui/skeleton'
import {
  getOracleStatusFn,
  issueOracleCredentialFn,
  revokeOracleCredentialFn,
} from '@/server/oracle-fn'
import { usePermissions } from '@/lib/hooks/use-permissions'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'

type IssuedCred = {
  username: string
  password: string
  mode: string
  lease_id?: string
  secret_ref?: string
  issued_at: string
}

export function OraclePage() {
  const { can } = usePermissions()
  const canManage = can('secrets:manage') || can('system:admin')
  const canRead =
    can('secrets:read') || can('secrets:manage') || can('system:admin')
  const qc = useQueryClient()

  const [mode, setMode] = useState<'auto' | 'openbao' | 'proxy'>('auto')
  const [storeKv, setStoreKv] = useState(true)
  const [prefix, setPrefix] = useState('v')
  const [issued, setIssued] = useState<IssuedCred | null>(null)
  const [history, setHistory] = useState<IssuedCred[]>([])
  const [revokeUser, setRevokeUser] = useState<string | null>(null)

  const status = useQuery({
    queryKey: ['oracle', 'status'],
    queryFn: () => getOracleStatusFn(),
    enabled: canRead,
    refetchInterval: 30_000,
  })

  const issue = useMutation({
    mutationFn: () =>
      issueOracleCredentialFn({
        data: {
          mode,
          username_prefix: prefix || 'v',
          store_in_kv: storeKv,
        },
      }),
    onSuccess: (res) => {
      const cred: IssuedCred = {
        username: res.username,
        password: res.password,
        mode: res.mode,
        lease_id: res.lease_id,
        secret_ref: res.secret_ref,
        issued_at: new Date().toISOString(),
      }
      setIssued(cred)
      setHistory((h) => [cred, ...h].slice(0, 20))
      toast.success(res.message)
      void qc.invalidateQueries({ queryKey: ['oracle'] })
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const revoke = useMutation({
    mutationFn: (username: string) => {
      const lease = history.find((h) => h.username === username)?.lease_id
      return revokeOracleCredentialFn({
        data: {
          username,
          lease_id: lease,
          mode: 'auto',
        },
      })
    },
    onSuccess: (res) => {
      toast.success(res.message)
      setRevokeUser(null)
      if (issued?.username === res.username) setIssued(null)
      setHistory((h) => h.filter((x) => x.username !== res.username))
    },
    onError: (e) => toast.error((e as Error).message),
  })

  if (!canRead) {
    return (
      <div className="p-6 text-sm text-muted-foreground">
        Sem permissão para Oracle / secrets.
      </div>
    )
  }

  if (status.isLoading) {
    return (
      <div className="p-6 space-y-3">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-40 w-full" />
      </div>
    )
  }

  const proxyOk = status.data?.proxy.ok
  const baoRole = status.data?.openbao

  return (
    <div className="space-y-6 p-6 max-w-4xl">
      <div>
        <h1 className="text-2xl font-bold flex items-center gap-2">
          <Database className="h-7 w-7 text-primary" />
          Oracle
        </h1>
        <p className="text-sm text-muted-foreground mt-1">
          Credenciais dinâmicas (ADR-001): OpenBao database role ou oracle-proxy
          direto. Senha exibida só no momento da emissão.
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">oracle-proxy</CardTitle>
            <CardDescription className="font-mono text-xs break-all">
              {status.data?.proxy.url}
            </CardDescription>
          </CardHeader>
          <CardContent className="flex items-center gap-2">
            {proxyOk ? (
              <Badge className="gap-1">
                <CheckCircle2 className="h-3 w-3" /> healthy
              </Badge>
            ) : (
              <Badge variant="destructive" className="gap-1">
                <AlertTriangle className="h-3 w-3" /> offline
              </Badge>
            )}
            <Button
              size="sm"
              variant="ghost"
              onClick={() => void status.refetch()}
            >
              <RefreshCw className="h-3.5 w-3.5" />
            </Button>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">OpenBao role</CardTitle>
            <CardDescription className="font-mono text-xs">
              database/creds/{baoRole?.role || 'oracle-readonly'}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {!baoRole?.configured ? (
              <Badge variant="secondary">token não configurado</Badge>
            ) : baoRole.reachable ? (
              <Badge className="gap-1">
                <CheckCircle2 className="h-3 w-3" /> role OK
              </Badge>
            ) : (
              <div className="space-y-1">
                <Badge variant="outline">role não registrada</Badge>
                <p className="text-xs text-muted-foreground">
                  {baoRole.error || 'Use mode proxy até registrar o plugin'}
                </p>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {status.data?.mock_hint && (
        <p className="text-xs text-muted-foreground flex items-start gap-2">
          <AlertTriangle className="h-3.5 w-3.5 shrink-0 mt-0.5" />
          {status.data.mock_hint}
        </p>
      )}

      {canManage && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base flex items-center gap-2">
              <KeyRound className="h-4 w-4" />
              Emitir credencial
            </CardTitle>
            <CardDescription>
              auto = tenta OpenBao e cai no proxy. store KV = grava em
              secret/data/archgate/targets/oracle_*
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1">
              <Label>Modo</Label>
              <Select
                value={mode}
                onValueChange={(v) => setMode(v as typeof mode)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="auto">auto (OpenBao → proxy)</SelectItem>
                  <SelectItem value="openbao">somente OpenBao</SelectItem>
                  <SelectItem value="proxy">somente oracle-proxy</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1">
              <Label>Prefixo username (≤8)</Label>
              <Input
                className="font-mono"
                value={prefix}
                maxLength={8}
                onChange={(e) => setPrefix(e.target.value)}
              />
            </div>
            <div className="flex items-center gap-2 sm:col-span-2">
              <Switch checked={storeKv} onCheckedChange={setStoreKv} />
              <Label>Guardar cópia no OpenBao KV (secret_ref)</Label>
            </div>
            <div className="sm:col-span-2">
              <Button
                disabled={issue.isPending || (!proxyOk && mode !== 'openbao')}
                onClick={() => issue.mutate()}
              >
                {issue.isPending ? 'Emitindo…' : 'Emitir credencial Oracle'}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {issued && (
        <Card className="border-emerald-500/40">
          <CardHeader>
            <CardTitle className="text-base">Credencial emitida</CardTitle>
            <CardDescription>
              Copie agora — a senha não será exibida de novo nesta sessão
              (exceto se estiver no histórico local do browser).
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="outline">{issued.mode}</Badge>
              {issued.lease_id && (
                <span className="font-mono text-xs text-muted-foreground">
                  lease: {issued.lease_id}
                </span>
              )}
            </div>
            <Row
              label="Username"
              value={issued.username}
              onCopy={() => {
                void navigator.clipboard.writeText(issued.username)
                toast.success('Username copiado')
              }}
            />
            <Row
              label="Password"
              value={issued.password}
              onCopy={() => {
                void navigator.clipboard.writeText(issued.password)
                toast.success('Password copiado')
              }}
            />
            {issued.secret_ref && (
              <Row
                label="secret_ref"
                value={issued.secret_ref}
                onCopy={() => {
                  void navigator.clipboard.writeText(issued.secret_ref!)
                  toast.success('Path copiado')
                }}
              />
            )}
            {canManage && (
              <Button
                variant="destructive"
                size="sm"
                onClick={() => setRevokeUser(issued.username)}
              >
                <Trash2 className="h-3.5 w-3.5 mr-1" />
                Revogar esta credencial
              </Button>
            )}
          </CardContent>
        </Card>
      )}

      {history.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Emitidas nesta sessão</CardTitle>
            <CardDescription>
              Histórico só no browser (não é inventário persistente).
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {history.map((h) => (
              <div
                key={h.username + h.issued_at}
                className="flex flex-wrap items-center justify-between gap-2 rounded border p-2 text-sm"
              >
                <div>
                  <span className="font-mono font-medium">{h.username}</span>
                  <span className="text-xs text-muted-foreground ml-2">
                    {h.mode} · {new Date(h.issued_at).toLocaleTimeString()}
                  </span>
                </div>
                {canManage && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setRevokeUser(h.username)}
                  >
                    Revogar
                  </Button>
                )}
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      <ConfirmDialog
        open={!!revokeUser}
        onOpenChange={(o) => !o && setRevokeUser(null)}
        title="Revogar credencial Oracle"
        description={`DROP USER / revoke lease para ${revokeUser}. Digite o username para confirmar.`}
        confirmText={revokeUser || ''}
        destructive
        isLoading={revoke.isPending}
        onConfirm={() => {
          if (revokeUser) revoke.mutate(revokeUser)
        }}
      />
    </div>
  )
}

function Row({
  label,
  value,
  onCopy,
}: {
  label: string
  value: string
  onCopy: () => void
}) {
  return (
    <div className="flex items-center justify-between gap-2 rounded bg-muted/50 px-3 py-2">
      <div className="min-w-0">
        <div className="text-xs text-muted-foreground">{label}</div>
        <div className="font-mono text-sm truncate">{value}</div>
      </div>
      <Button size="icon" variant="ghost" onClick={onCopy}>
        <Copy className="h-4 w-4" />
      </Button>
    </div>
  )
}
