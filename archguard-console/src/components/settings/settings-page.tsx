// src/components/settings/settings-page.tsx

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Settings,
  Shield,
  Database,
  Server,
  Globe,
  Lock,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { PermissionGate } from '@/components/shared/permission-gate'
import { CopyButton } from '@/components/shared/copy-button'
import { systemApi } from '@/lib/api/kanidm-client'
import { queryKeys } from '@/lib/utils/query-keys'

export function SettingsPage() {
  const { data: systemStatus, isLoading: statusLoading } = useQuery({
    queryKey: queryKeys.system.status,
    queryFn: () => systemApi.status(),
    staleTime: 30_000,
  })

  const { data: domainInfo, isLoading: domainLoading } = useQuery({
    queryKey: queryKeys.system.domain,
    queryFn: () => systemApi.domain(),
    staleTime: 60_000,
  })

  const isLoading = statusLoading || domainLoading

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Configurações</h1>
        <p className="text-muted-foreground">
          Visualize configurações do sistema e políticas de segurança
        </p>
      </div>

      <Tabs defaultValue="general">
        <TabsList>
          <TabsTrigger value="general">
            <Settings className="mr-2 h-4 w-4" />
            Geral
          </TabsTrigger>
          <TabsTrigger value="security">
            <Shield className="mr-2 h-4 w-4" />
            Segurança
          </TabsTrigger>
          <TabsTrigger value="backup">
            <Database className="mr-2 h-4 w-4" />
            Backup
          </TabsTrigger>
          <TabsTrigger value="system">
            <Server className="mr-2 h-4 w-4" />
            Sistema
          </TabsTrigger>
        </TabsList>

        <TabsContent value="general" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base flex items-center gap-2">
                <Globe className="h-4 w-4" />
                Informações do Domínio
              </CardTitle>
              <CardDescription>
                Configurações do domínio Kanidm
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {isLoading ? (
                <SettingsFieldsSkeleton count={4} />
              ) : (
                <>
                  <SettingsField
                    label="Domínio"
                    value={
                      domainInfo &&
                      typeof domainInfo === 'object' &&
                      'attrs' in (domainInfo as Record<string, unknown>)
                        ? ((domainInfo as Record<string, Record<string, string[]>>).attrs?.['domain_name']?.[0] ?? '—')
                        : '—'
                    }
                  />
                  <SettingsField
                    label="Display Name"
                    value={
                      domainInfo &&
                      typeof domainInfo === 'object' &&
                      'attrs' in (domainInfo as Record<string, unknown>)
                        ? ((domainInfo as Record<string, Record<string, string[]>>).attrs?.['domain_display_name']?.[0] ?? '—')
                        : '—'
                    }
                  />
                  <SettingsField
                    label="Status"
                    value=""
                  >
                    <Badge variant="default">
                      {systemStatus &&
                      typeof systemStatus === 'object' &&
                      'state' in (systemStatus as Record<string, unknown>) &&
                      (systemStatus as Record<string, string>).state === 'ok'
                        ? 'Online'
                        : 'Indisponível'}
                    </Badge>
                  </SettingsField>
                  <SettingsField
                    label="Versão Kanidm"
                    value={
                      systemStatus &&
                      typeof systemStatus === 'object' &&
                      'version' in (systemStatus as Record<string, unknown>)
                        ? String((systemStatus as Record<string, string>).version)
                        : '—'
                    }
                  />
                </>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="security" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base flex items-center gap-2">
                <Lock className="h-4 w-4" />
                Políticas de Autenticação
              </CardTitle>
              <CardDescription>
                Configurações de MFA e credenciais (leitura do Kanidm)
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <SettingsField
                label="Método Mínimo"
                value=""
              >
                <Badge variant="secondary">password_mfa</Badge>
              </SettingsField>
              <SettingsField
                label="Passkeys Habilitadas"
                value=""
              >
                <Badge variant="default">Sim</Badge>
              </SettingsField>
              <SettingsField
                label="TOTP Habilitado"
                value=""
              >
                <Badge variant="default">Sim</Badge>
              </SettingsField>
              <SettingsField
                label="Backup Codes"
                value=""
              >
                <Badge variant="default">Sim</Badge>
              </SettingsField>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Políticas de Sessão</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <SettingsField label="Timeout de Sessão" value="8 horas" />
              <SettingsField label="Lockout após" value="5 tentativas" />
              <SettingsField label="Duração do Lockout" value="30 minutos" />
            </CardContent>
          </Card>

          <PermissionGate require="settings:security">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Contas Break-Glass</CardTitle>
                <CardDescription>
                  Contas de emergência para acesso em caso de falha de autenticação
                </CardDescription>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">
                  Configuração de contas break-glass deve ser feita via CLI do Kanidm.
                  Consulte a documentação para mais detalhes.
                </p>
              </CardContent>
            </Card>
          </PermissionGate>
        </TabsContent>

        <TabsContent value="backup" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base flex items-center gap-2">
                <Database className="h-4 w-4" />
                Backup do Kanidm
              </CardTitle>
              <CardDescription>
                Status e configuração de backups
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <SettingsField label="Agendamento" value="Diário às 02:00" />
              <SettingsField label="Retenção" value="30 dias" />
              <SettingsField
                label="Último Backup"
                value=""
              >
                <Badge variant="default">Sucesso</Badge>
              </SettingsField>
              <Separator />
              <p className="text-sm text-muted-foreground">
                Backups são gerenciados pelo servidor Kanidm. Para fazer download
                ou restauração manual, use o CLI do Kanidm.
              </p>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="system" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Informações do Console</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <SettingsField label="ArchGuard Console" value="v1.0.0" />
              <SettingsField label="Framework" value="TanStack Start" />
              <SettingsField label="Node.js" value="v22" />
              <SettingsField
                label="Ambiente"
                value={
                  typeof process !== 'undefined' && process.env?.NODE_ENV === 'production'
                    ? 'Produção'
                    : 'Desenvolvimento'
                }
              />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Variáveis de Ambiente</CardTitle>
              <CardDescription>
                Configurações carregadas (valores ocultados)
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <SettingsField label="ARCHGUARD_ID_URL" value="***configurado***" />
              <SettingsField label="ARCHGUARD_SA_TOKEN" value="***configurado***" />
              <SettingsField label="ARCHGUARD_VAULT_URL" value="***configurado***" />
              <SettingsField label="SESSION_SECRET" value="***configurado***" />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}

function SettingsField({
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

function SettingsFieldsSkeleton({ count }: { count: number }) {
  return (
    <>
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="flex items-center justify-between">
          <Skeleton className="h-4 w-24" />
          <Skeleton className="h-4 w-32" />
        </div>
      ))}
    </>
  )
}
