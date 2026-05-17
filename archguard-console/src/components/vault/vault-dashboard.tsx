// src/components/vault/vault-dashboard.tsx

import { useQuery } from '@tanstack/react-query'
import {
  Vault,
  Shield,
  Mail,
  Users,
  Key,
  ExternalLink,
  CheckCircle2,
  XCircle,
  RefreshCw,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { vaultApi } from '@/lib/api/vault-client'
import { queryKeys } from '@/lib/utils/query-keys'

export function VaultDashboard() {
  const {
    data: status,
    isLoading,
    refetch,
  } = useQuery({
    queryKey: queryKeys.vault.status,
    queryFn: () => vaultApi.status(),
    staleTime: 30_000,
    refetchInterval: 30_000,
  })

  if (isLoading) {
    return <VaultSkeleton />
  }

  const isOnline = status?.online ?? false

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">ArchGuard Vault</h1>
          <p className="text-muted-foreground">
            Status e informações do cofre de senhas
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            <RefreshCw className="mr-2 h-4 w-4" />
            Atualizar
          </Button>
          <Button variant="outline" size="sm" asChild>
            <a
              href={
                typeof window !== 'undefined'
                  ? `${window.location.origin}/vault`
                  : '#'
              }
              target="_blank"
              rel="noopener noreferrer"
            >
              <ExternalLink className="mr-2 h-4 w-4" />
              Abrir Vault UI
            </a>
          </Button>
        </div>
      </div>

      {/* Status Banner */}
      <Card className={isOnline ? 'border-green-500/30' : 'border-destructive/30'}>
        <CardContent className="flex items-center gap-4 pt-6">
          {isOnline ? (
            <CheckCircle2 className="h-10 w-10 text-green-500" />
          ) : (
            <XCircle className="h-10 w-10 text-destructive" />
          )}
          <div>
            <h2 className="text-lg font-semibold">
              {isOnline ? 'Vault Online' : 'Vault Offline'}
            </h2>
            <p className="text-sm text-muted-foreground">
              {isOnline
                ? 'O cofre de senhas está funcionando normalmente'
                : 'O cofre de senhas está indisponível'}
            </p>
          </div>
          <Badge
            variant={isOnline ? 'default' : 'destructive'}
            className="ml-auto"
          >
            {isOnline ? 'Online' : 'Offline'}
          </Badge>
        </CardContent>
      </Card>

      {/* Stats Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Status
            </CardTitle>
            <Shield className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">
              {isOnline ? 'Online' : 'Offline'}
            </p>
            {status?.version && (
              <p className="text-xs text-muted-foreground">
                v{status.version}
              </p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Vaults
            </CardTitle>
            <Vault className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{status?.totalVaults ?? 0}</p>
            <p className="text-xs text-muted-foreground">Cofres criados</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Aliases Ativos
            </CardTitle>
            <Users className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">
              {status?.activeAliases ?? 0}
            </p>
            <p className="text-xs text-muted-foreground">Email aliases</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Senhas
            </CardTitle>
            <Key className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">
              {status?.totalPasswords ?? 0}
            </p>
            <p className="text-xs text-muted-foreground">
              Credenciais armazenadas
            </p>
          </CardContent>
        </Card>
      </div>

      {/* SMTP Status */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base flex items-center gap-2">
            <Mail className="h-4 w-4" />
            Status SMTP
          </CardTitle>
          <CardDescription>
            Configuração de email para aliases do Vault
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <SmtpRow
            label="Servidor SMTP"
            ok={status?.smtp?.online ?? false}
          />
          <SmtpRow
            label="MX Configurado"
            ok={status?.smtp?.mxConfigured ?? false}
          />
          <SmtpRow
            label="SPF Válido"
            ok={status?.smtp?.spfValid ?? false}
          />
          <SmtpRow
            label="DKIM Válido"
            ok={status?.smtp?.dkimValid ?? false}
          />
        </CardContent>
      </Card>

      {/* Info */}
      <Card>
        <CardContent className="pt-6">
          <p className="text-sm text-muted-foreground">
            O ArchGuard Vault é baseado no AliasVault e oferece criptografia
            zero-knowledge para senhas e aliases de email. A administração
            detalhada de vaults individuais é feita através da interface
            dedicada do Vault.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}

function SmtpRow({ label, ok }: { label: string; ok: boolean }) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-muted-foreground">{label}</span>
      {ok ? (
        <Badge variant="default" className="gap-1">
          <CheckCircle2 className="h-3 w-3" />
          OK
        </Badge>
      ) : (
        <Badge variant="secondary" className="gap-1">
          <XCircle className="h-3 w-3" />
          Não configurado
        </Badge>
      )}
    </div>
  )
}

function VaultSkeleton() {
  return (
    <div className="space-y-6">
      <div>
        <Skeleton className="h-9 w-56" />
        <Skeleton className="mt-2 h-5 w-72" />
      </div>
      <Skeleton className="h-24" />
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-28" />
        ))}
      </div>
      <Skeleton className="h-40" />
    </div>
  )
}
