// src/components/oauth2/oauth2-detail-page.tsx

import { useState } from 'react'
import { useNavigate, Link } from '@tanstack/react-router'
import { Route } from '@/routes/_authed/oauth2/$clientId'
import {
  ArrowLeft,
  Settings,
  Globe,
  Shield,
  Code,
  Trash2,
  Plus,
  X,
  Copy,
  AlertTriangle,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { PermissionGate } from '@/components/shared/permission-gate'
import { CopyButton } from '@/components/shared/copy-button'
import {
  useOAuth2Client,
  useDeleteOAuth2Client,
  useSetScopeMap,
  useDeleteScopeMap,
  useAddRedirectUrl,
} from '@/lib/hooks/use-oauth2'
import { useGroups } from '@/lib/hooks/use-groups'
import { getIntegrationSnippets } from '@/components/oauth2/integration-snippets'

export function OAuth2DetailPage() {
  const { clientId } = Route.useParams()
  const navigate = useNavigate()
  const { data: client, isLoading } = useOAuth2Client(clientId)
  const { data: groups } = useGroups()
  const deleteClient = useDeleteOAuth2Client()
  const addScopeMap = useSetScopeMap()
  const removeScopeMap = useDeleteScopeMap()
  const addRedirectUrl = useAddRedirectUrl()

  const [showDelete, setShowDelete] = useState(false)
  const [newRedirectUrl, setNewRedirectUrl] = useState('')
  const [scopeGroupId, setScopeGroupId] = useState('')
  const [scopeInput, setScopeInput] = useState('')

  if (isLoading || !client) {
    return <OAuth2DetailSkeleton />
  }

  const handleAddRedirectUrl = () => {
    if (newRedirectUrl) {
      addRedirectUrl.mutate(
        { clientId: client.id, url: newRedirectUrl },
        { onSuccess: () => setNewRedirectUrl('') },
      )
    }
  }

  const handleAddScopeMap = () => {
    if (scopeGroupId && scopeInput) {
      const scopes = scopeInput.split(',').map((s) => s.trim()).filter(Boolean)
      addScopeMap.mutate(
        { clientId: client.id, groupId: scopeGroupId, scopes },
        {
          onSuccess: () => {
            setScopeGroupId('')
            setScopeInput('')
          },
        },
      )
    }
  }

  const snippets = getIntegrationSnippets(client)

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/oauth2">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex items-center gap-2">
          {client.type === 'public' ? (
            <Globe className="h-6 w-6 text-blue-500" />
          ) : (
            <Shield className="h-6 w-6 text-green-500" />
          )}
          <div className="flex-1">
            <h1 className="text-2xl font-bold">{client.displayName}</h1>
            <p className="font-mono text-sm text-muted-foreground">
              {client.name}
            </p>
          </div>
        </div>
        <Badge variant={client.type === 'public' ? 'outline' : 'secondary'}>
          {client.type === 'public' ? 'Public (PKCE)' : 'Basic (Confidential)'}
        </Badge>
      </div>

      <Tabs defaultValue="config">
        <TabsList>
          <TabsTrigger value="config">
            <Settings className="mr-2 h-4 w-4" />
            Configuração
          </TabsTrigger>
          <TabsTrigger value="scopes">
            <Globe className="mr-2 h-4 w-4" />
            Scope Maps
          </TabsTrigger>
          <TabsTrigger value="snippets">
            <Code className="mr-2 h-4 w-4" />
            Integração
          </TabsTrigger>
          <TabsTrigger value="danger">
            <AlertTriangle className="mr-2 h-4 w-4" />
            Danger Zone
          </TabsTrigger>
        </TabsList>

        <TabsContent value="config" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Informações</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <InfoRow label="Client ID" value={client.name}>
                <CopyButton text={client.name} />
              </InfoRow>
              <InfoRow label="ID" value={client.id}>
                <CopyButton text={client.id} />
              </InfoRow>
              <InfoRow label="Tipo" value={client.type === 'public' ? 'Public' : 'Basic'} />
              <InfoRow label="Landing URL" value={client.landingUrl} />
              <InfoRow label="PKCE" value={client.isPkceEnabled ? 'Habilitado' : 'Desabilitado'} />
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="text-base">Redirect URLs</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {client.redirectUrls.map((url) => (
                <div
                  key={url}
                  className="flex items-center justify-between rounded border p-2 text-sm"
                >
                  <span className="font-mono">{url}</span>
                  <CopyButton text={url} />
                </div>
              ))}
              <PermissionGate require="oauth2:update">
                <div className="flex gap-2">
                  <Input
                    value={newRedirectUrl}
                    onChange={(e) => setNewRedirectUrl(e.target.value)}
                    placeholder="https://app.exemplo.com/callback"
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.preventDefault()
                        handleAddRedirectUrl()
                      }
                    }}
                  />
                  <Button
                    variant="outline"
                    onClick={handleAddRedirectUrl}
                    disabled={addRedirectUrl.isPending}
                  >
                    <Plus className="mr-2 h-4 w-4" />
                    Adicionar
                  </Button>
                </div>
              </PermissionGate>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="scopes" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Scope Maps</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {client.scopeMaps.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  Nenhum scope map configurado
                </p>
              ) : (
                client.scopeMaps.map((sm) => (
                  <div
                    key={sm.groupId}
                    className="flex items-center justify-between rounded-lg border p-3"
                  >
                    <div>
                      <p className="text-sm font-medium">{sm.groupName}</p>
                      <div className="flex gap-1 mt-1">
                        {sm.scopes.map((s) => (
                          <Badge key={s} variant="outline" className="text-xs">
                            {s}
                          </Badge>
                        ))}
                      </div>
                    </div>
                    <PermissionGate require="oauth2:update">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8"
                        onClick={() =>
                          removeScopeMap.mutate({
                            clientId: client.id,
                            groupId: sm.groupId,
                          })
                        }
                      >
                        <X className="h-4 w-4" />
                      </Button>
                    </PermissionGate>
                  </div>
                ))
              )}

              <PermissionGate require="oauth2:update">
                <Separator />
                <div className="grid gap-2 md:grid-cols-[1fr_1fr_auto]">
                  <select
                    className="rounded-md border bg-background px-3 py-2 text-sm"
                    value={scopeGroupId}
                    onChange={(e) => setScopeGroupId(e.target.value)}
                  >
                    <option value="">Selecione grupo</option>
                    {groups
                      ?.filter((g) => !g.isBuiltin)
                      .map((g) => (
                        <option key={g.id} value={g.id}>
                          {g.name}
                        </option>
                      ))}
                  </select>
                  <Input
                    value={scopeInput}
                    onChange={(e) => setScopeInput(e.target.value)}
                    placeholder="openid, profile, email"
                  />
                  <Button variant="outline" onClick={handleAddScopeMap}>
                    <Plus className="mr-2 h-4 w-4" />
                    Adicionar
                  </Button>
                </div>
              </PermissionGate>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="snippets">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Snippets de Integração</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {snippets.map((snippet) => (
                <div key={snippet.framework} className="space-y-2">
                  <div className="flex items-center justify-between">
                    <Label className="font-medium">{snippet.framework}</Label>
                    <CopyButton text={snippet.code} />
                  </div>
                  <pre className="overflow-x-auto rounded-lg border bg-muted p-4 text-xs">
                    <code>{snippet.code}</code>
                  </pre>
                </div>
              ))}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="danger">
          <Card className="border-destructive">
            <CardHeader>
              <CardTitle className="text-base text-destructive">
                Zona de Perigo
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <PermissionGate require="oauth2:delete">
                <div className="flex items-center justify-between rounded-lg border border-destructive/50 p-4">
                  <div>
                    <p className="font-medium">Excluir Client</p>
                    <p className="text-sm text-muted-foreground">
                      Remove permanentemente este cliente. Integrações deixarão de funcionar.
                    </p>
                  </div>
                  <Button
                    variant="destructive"
                    onClick={() => setShowDelete(true)}
                  >
                    <Trash2 className="mr-2 h-4 w-4" />
                    Excluir
                  </Button>
                </div>
              </PermissionGate>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      <ConfirmDialog
        open={showDelete}
        onOpenChange={setShowDelete}
        title="Excluir Cliente OAuth2"
        description={`Esta ação é irreversível. O cliente "${client.displayName}" será removido.`}
        confirmText={client.name}
        destructive
        isLoading={deleteClient.isPending}
        onConfirm={() => {
          deleteClient.mutate(client.id, {
            onSuccess: () => navigate({ to: '/oauth2' }),
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
        <span className="font-medium">{value}</span>
        {children}
      </div>
    </div>
  )
}

function OAuth2DetailSkeleton() {
  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Skeleton className="h-10 w-10" />
        <Skeleton className="h-6 w-6 rounded" />
        <div className="flex-1">
          <Skeleton className="h-8 w-48" />
          <Skeleton className="mt-1 h-4 w-32" />
        </div>
      </div>
      <Skeleton className="h-10 w-96" />
      <Skeleton className="h-64" />
    </div>
  )
}
