// src/components/vault/vault-dashboard.tsx
// OpenBao status surface (ArchGate secrets plane). Replaces AliasVault stub.

import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import {
  Vault,
  Shield,
  Key,
  ExternalLink,
  CheckCircle2,
  XCircle,
  RefreshCw,
  AlertTriangle,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { vaultApi } from '@/lib/api/vault-client'
import { queryKeys } from '@/lib/utils/query-keys'

export function VaultDashboard() {
  const { t } = useTranslation()
  const {
    data: status,
    isLoading,
    refetch,
    isFetching,
  } = useQuery({
    queryKey: queryKeys.vault.status,
    queryFn: () => vaultApi.status(),
    staleTime: 15_000,
    refetchInterval: 30_000,
  })

  if (isLoading) {
    return <VaultSkeleton />
  }

  const isOnline = status?.online ?? false
  const sealed = (status as { sealed?: boolean } | undefined)?.sealed
  const initialized = (status as { initialized?: boolean } | undefined)?.initialized
  const cluster = (status as { cluster?: string } | undefined)?.cluster
  const addr = (status as { addr?: string } | undefined)?.addr
  const tokenOk = (status as { token_configured?: boolean } | undefined)?.token_configured
  const tokenKind = (status as { token_kind?: string } | undefined)?.token_kind
  const err = (status as { error?: string } | undefined)?.error

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">{t('vaultPage.title')}</h1>
          <p className="text-muted-foreground">
            {t('vaultPage.subtitle')}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => void refetch()}
            disabled={isFetching}
          >
            <RefreshCw className={`mr-2 h-4 w-4 ${isFetching ? 'animate-spin' : ''}`} />
            Atualizar
          </Button>
          <Button variant="outline" size="sm" asChild>
            <Link to="/secrets">
              <Key className="mr-2 h-4 w-4" />
              {t('vaultPage.secretsLink')}
            </Link>
          </Button>
          <Button variant="outline" size="sm" asChild>
            <a
              href="https://secrets.archgate.com.br"
              target="_blank"
              rel="noopener noreferrer"
            >
              <ExternalLink className="mr-2 h-4 w-4" />
              {t('vaultPage.openUi')}
            </a>
          </Button>
        </div>
      </div>

      <Card className={isOnline ? 'border-green-500/30' : 'border-destructive/30'}>
        <CardContent className="flex items-center gap-4 pt-6">
          {isOnline ? (
            <CheckCircle2 className="h-10 w-10 text-green-500" />
          ) : (
            <XCircle className="h-10 w-10 text-destructive" />
          )}
          <div className="min-w-0 flex-1">
            <h2 className="text-lg font-semibold">
              {isOnline ? t('vaultPage.online') : t('vaultPage.offline')}
            </h2>
            <p className="text-sm text-muted-foreground">
              {isOnline
                ? t('vaultPage.onlineHint')
                : err
                  ? err
                  : t('vaultPage.offlineHint')}
            </p>
            {addr && (
              <p className="text-xs font-mono text-muted-foreground mt-1 truncate">
                {addr}
              </p>
            )}
          </div>
          <Badge
            variant={isOnline ? 'default' : 'destructive'}
            className="ml-auto shrink-0"
          >
            {isOnline ? t('common.online') : t('common.offline')}
          </Badge>
        </CardContent>
      </Card>

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
              {isOnline ? t('common.online') : t('common.offline')}
            </p>
            {status?.version && (
              <p className="text-xs text-muted-foreground">v{status.version}</p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Seal
            </CardTitle>
            <Vault className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">
              {sealed ? 'Sealed' : initialized === false ? 'N/A' : 'Unsealed'}
            </p>
            <p className="text-xs text-muted-foreground">
              {initialized ? 'initialized' : 'not initialized'}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t('vaultPage.tokenApi')}
            </CardTitle>
            <Key className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{tokenOk ? t('vaultPage.tokenOk') : t('vaultPage.tokenAbsent')}</p>
            <p className="text-xs text-muted-foreground">
              {tokenKind ? `kind: ${tokenKind}` : 'OPENBAO_*_TOKEN'}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Cluster
            </CardTitle>
            <Shield className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <p className="text-lg font-bold truncate" title={cluster || ''}>
              {cluster || '—'}
            </p>
            <p className="text-xs text-muted-foreground">OpenBao cluster</p>
          </CardContent>
        </Card>
      </div>

      {!isOnline && (
        <Card className="border-amber-500/30">
          <CardContent className="flex gap-3 pt-6 text-sm text-muted-foreground">
            <AlertTriangle className="h-5 w-5 text-amber-500 shrink-0" />
            <div>
              <p className="font-medium text-foreground">{t('vaultPage.troubleshooting')}</p>
              <ul className="mt-1 list-disc pl-4 space-y-1">
                <li>
                  Confirme <code className="text-xs">OPENBAO_ADDR</code> e token no
                  container do console
                </li>
                <li>Se sealed, use unseal em {t('vaultPage.secretsLink')} (ou OPENBAO_UNSEAL_KEY)</li>
                <li>
                  Detalhes de mounts/leases em{' '}
                  <Link to="/secrets" className="text-primary underline">
                    {t('vaultPage.secretsLink')}
                  </Link>
                </li>
              </ul>
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t('vaultPage.about')}</CardTitle>
          <CardDescription>
            O cofre ArchGate é o <strong>OpenBao</strong> (compatível com Vault API).
            Senhas e tokens de aplicação ficam no servidor; o browser só vê status.
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>
            Para engines, auth JWT e leases dinâmicos, use o módulo{' '}
            <Link to="/secrets" className="text-primary underline">
              {t('vaultPage.secretsLink')}
            </Link>
            . A UI stock (break-glass) fica em secrets.archgate.com.br quando
            exposta.
          </p>
        </CardContent>
      </Card>
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
