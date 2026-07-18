// src/components/service-account/sa-detail-page.tsx

import { useState } from 'react'
import { useNavigate, Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { enumLabel } from '@/lib/i18n/labels'
import { Route } from '@/routes/_authed/service-accounts/$accountId'
import {
  ArrowLeft,
  Bot,
  Key,
  Users,
  Settings,
  Trash2,
  Plus,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Skeleton } from '@/components/ui/skeleton'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { StatusBadge } from '@/components/shared/status-badge'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { PermissionGate } from '@/components/shared/permission-gate'
import { CopyButton } from '@/components/shared/copy-button'
import { GroupBadge } from '@/components/shared/group-badge'
import { TimeAgo } from '@/components/shared/time-ago'
import {
  useServiceAccount,
  useDeleteServiceAccount,
  useGenerateApiToken,
  useRevokeApiToken,
} from '@/lib/hooks/use-service-accounts'

export function ServiceAccountDetailPage() {
  const { t } = useTranslation()
  const { accountId } = Route.useParams()
  const navigate = useNavigate()
  const { data: account, isLoading } = useServiceAccount(accountId)
  const deleteAccount = useDeleteServiceAccount()
  const generateToken = useGenerateApiToken()
  const revokeToken = useRevokeApiToken()

  const [showDelete, setShowDelete] = useState(false)
  const [showGenerateToken, setShowGenerateToken] = useState(false)
  const [tokenLabel, setTokenLabel] = useState('')
  const [tokenExpiry, setTokenExpiry] = useState('never')
  const [generatedToken, setGeneratedToken] = useState<string | null>(null)

  if (isLoading || !account) {
    return <SADetailSkeleton />
  }

  const handleGenerateToken = () => {
    const expiry = tokenExpiry === 'never' ? undefined : tokenExpiry
    generateToken.mutate(
      { id: accountId, label: tokenLabel, expiry },
      {
        onSuccess: (data) => {
          setGeneratedToken(data.token)
        },
      },
    )
  }

  const handleCloseTokenDialog = () => {
    setShowGenerateToken(false)
    setGeneratedToken(null)
    setTokenLabel('')
    setTokenExpiry('never')
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/service-accounts">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <Bot className="h-8 w-8 text-muted-foreground" />
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <h1 className="text-2xl font-bold">{account.displayName}</h1>
            <StatusBadge status={account.status as 'active' | 'expired' | 'disabled'} />
          </div>
          <p className="text-muted-foreground">@{account.name}</p>
        </div>
        <PermissionGate require="service_accounts:delete">
          <Button variant="destructive" onClick={() => setShowDelete(true)}>
            <Trash2 className="mr-2 h-4 w-4" />
            Excluir
          </Button>
        </PermissionGate>
      </div>

      <Tabs defaultValue="tokens">
        <TabsList>
          <TabsTrigger value="tokens">
            <Key className="mr-2 h-4 w-4" />
            API Tokens
          </TabsTrigger>
          <TabsTrigger value="groups">
            <Users className="mr-2 h-4 w-4" />
            Grupos
          </TabsTrigger>
          <TabsTrigger value="config">
            <Settings className="mr-2 h-4 w-4" />
            Configuração
          </TabsTrigger>
        </TabsList>

        <TabsContent value="tokens" className="space-y-4">
          <div className="flex items-center justify-between">
            <p className="text-sm text-muted-foreground">
              {account.apiTokens.length} token(s) ativo(s)
            </p>
            <PermissionGate require="service_accounts:tokens">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setShowGenerateToken(true)}
              >
                <Plus className="mr-2 h-4 w-4" />
                Gerar Token
              </Button>
            </PermissionGate>
          </div>

          {account.apiTokens.length === 0 ? (
            <Card>
              <CardContent className="py-8 text-center">
                <Key className="mx-auto mb-2 h-8 w-8 text-muted-foreground/50" />
                <p className="text-sm text-muted-foreground">
                  Nenhum token de API gerado
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="space-y-2">
              {account.apiTokens.map((token) => (
                <div
                  key={token.tokenId}
                  className="flex items-center justify-between rounded-lg border p-3"
                >
                  <div>
                    <p className="text-sm font-medium">{token.label}</p>
                    <div className="flex items-center gap-2 text-xs text-muted-foreground">
                      <span>
                        Criado: <TimeAgo date={token.createdAt} />
                      </span>
                      {token.expiresAt && (
                        <>
                          <span>·</span>
                          <span>
                            Expira: <TimeAgo date={token.expiresAt} />
                          </span>
                        </>
                      )}
                    </div>
                  </div>
                  <PermissionGate require="service_accounts:tokens">
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-destructive"
                      onClick={() =>
                        revokeToken.mutate({
                          id: accountId,
                          tokenId: token.tokenId,
                        })
                      }
                      disabled={revokeToken.isPending}
                    >
                      Revogar
                    </Button>
                  </PermissionGate>
                </div>
              ))}
            </div>
          )}
        </TabsContent>

        <TabsContent value="groups">
          <Card>
            <CardContent className="pt-6">
              {account.groupNames.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  Este service account não pertence a nenhum grupo
                </p>
              ) : (
                <div className="space-y-2">
                  {account.groupNames.map((g) => (
                    <div
                      key={g}
                      className="flex items-center justify-between rounded-lg border p-3"
                    >
                      <GroupBadge name={g} />
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="config">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t('common.info')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <InfoRow label="ID" value={account.id}>
                <CopyButton value={account.id} />
              </InfoRow>
              <InfoRow label="SPN" value={account.name}>
                <CopyButton value={account.name} />
              </InfoRow>
              <InfoRow label="Nome" value={account.displayName} />
              <InfoRow
                label={t('common.description')}
                value={account.description ?? t('common.noDescription')}
              />
              <InfoRow label="Status" value="">
                <StatusBadge status={account.status as 'active' | 'expired' | 'disabled'} />
              </InfoRow>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Generate Token Dialog */}
      <Dialog open={showGenerateToken} onOpenChange={handleCloseTokenDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('serviceAccounts.generateToken')}</DialogTitle>
          </DialogHeader>

          {!generatedToken ? (
            <>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="token-label">{t('serviceAccounts.tokenLabel')}</Label>
                  <Input
                    id="token-label"
                    value={tokenLabel}
                    onChange={(e) => setTokenLabel(e.target.value)}
                    placeholder="ci-pipeline-token"
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="token-expiry">{t('serviceAccounts.tokenExpiry')}</Label>
                  <Select value={tokenExpiry} onValueChange={setTokenExpiry}>
                    <SelectTrigger id="token-expiry">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {(
                        ['never', '30d', '90d', '180d', '365d'] as const
                      ).map((v) => (
                        <SelectItem key={v} value={v}>
                          {enumLabel(t, 'tokenExpiry', v)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={handleCloseTokenDialog}>
                  {t('common.cancel')}
                </Button>
                <Button
                  onClick={handleGenerateToken}
                  disabled={!tokenLabel || generateToken.isPending}
                >
                  {generateToken.isPending ? t('common.generating') : t('serviceAccounts.generate')}
                </Button>
              </DialogFooter>
            </>
          ) : (
            <>
              <div className="space-y-4 py-4">
                <div className="rounded-lg border bg-muted p-4">
                  <Label className="mb-2 block text-xs text-muted-foreground">
                    Token de API (copie agora — não será exibido novamente)
                  </Label>
                  <div className="flex items-center gap-2">
                    <Input
                      readOnly
                      value={generatedToken}
                      className="font-mono text-xs"
                    />
                    <CopyButton value={generatedToken} />
                  </div>
                </div>
                <p className="text-sm text-muted-foreground">
                  Armazene este token de forma segura. Ele fornece acesso direto
                  à API do Kanidm com as permissões deste service account.
                </p>
              </div>
              <DialogFooter>
                <Button onClick={handleCloseTokenDialog}>Fechar</Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation */}
      <ConfirmDialog
        open={showDelete}
        onOpenChange={setShowDelete}
        title={t('serviceAccounts.deleteTitle')}
        description={`Esta ação é irreversível. O service account "${account.displayName}" e todos os seus tokens serão removidos.`}
        confirmText={account.name}
        destructive
        isLoading={deleteAccount.isPending}
        onConfirm={() => {
          deleteAccount.mutate(account.id, {
            onSuccess: () => navigate({ to: '/service-accounts' }),
          })
        }}
      />
    </div>
  )
}

function InfoRow({
  label,
  value,
  children,
}: {
  label: string
  value: string
  children?: React.ReactNode
}) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-muted-foreground">{label}</span>
      <div className="flex items-center gap-2">
        {value && <span className="font-medium">{value}</span>}
        {children}
      </div>
    </div>
  )
}

function SADetailSkeleton() {
  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Skeleton className="h-10 w-10" />
        <Skeleton className="h-8 w-8" />
        <div className="flex-1">
          <Skeleton className="h-8 w-48" />
          <Skeleton className="mt-1 h-4 w-32" />
        </div>
      </div>
      <Skeleton className="h-10 w-64" />
      <div className="space-y-2">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-16" />
        ))}
      </div>
    </div>
  )
}
