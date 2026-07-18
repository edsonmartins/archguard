// src/components/settings/settings-page.tsx

import {
  Settings,
  Shield,
  Server,
  Globe,
  Lock,
  ExternalLink,
} from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { PermissionGate } from '@/components/shared/permission-gate'
import { systemApi, accountPolicyApi } from '@/lib/api/kanidm-client'
import { queryKeys } from '@/lib/utils/query-keys'
import { LanguageSwitcher } from '@/components/layout/language-switcher'
import { enumLabel } from '@/lib/i18n/labels'

const CREDENTIAL_VARIANT: Record<string, 'default' | 'secondary' | 'outline' | 'destructive'> = {
  any: 'outline',
  mfa: 'secondary',
  passkey: 'default',
  attested_passkey: 'default',
}

function formatSeconds(s: number | undefined): string {
  if (!s) return '—'
  if (s >= 86400) return `${Math.round(s / 86400)} dia(s)`
  if (s >= 3600) return `${Math.round(s / 3600)} hora(s)`
  return `${Math.round(s / 60)} minuto(s)`
}

export function SettingsPage() {
  const { t } = useTranslation()
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

  // Read account policy from idm_all_persons (global default policy group)
  const { data: accountPolicy, isLoading: policyLoading } = useQuery({
    queryKey: ['accountPolicy', 'idm_all_persons'],
    queryFn: () => accountPolicyApi.get('idm_all_persons'),
    staleTime: 60_000,
  })

  const isLoading = statusLoading || domainLoading

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">{t('settings.title')}</h1>
        <p className="text-muted-foreground">{t('settings.subtitle')}</p>
      </div>

      <Tabs defaultValue="general">
        <TabsList>
          <TabsTrigger value="general">
            <Settings className="mr-2 h-4 w-4" />
            {t('settings.general')}
          </TabsTrigger>
          <TabsTrigger value="security">
            <Shield className="mr-2 h-4 w-4" />
            {t('settings.security')}
          </TabsTrigger>
          <TabsTrigger value="system">
            <Server className="mr-2 h-4 w-4" />
            {t('settings.system')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="general" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base flex items-center gap-2">
                <Globe className="h-4 w-4" />
                {t('settings.language')}
              </CardTitle>
              <CardDescription>{t('settings.languageHint')}</CardDescription>
            </CardHeader>
            <CardContent>
              <LanguageSwitcher compact={false} />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base flex items-center gap-2">
                <Globe className="h-4 w-4" />
                {t('settings.domainInfo')}
              </CardTitle>
              <CardDescription>
                {t('settings.domainHint')}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {isLoading ? (
                <SettingsFieldsSkeleton count={4} />
              ) : (
                <>
                  <SettingsField
                    label={t('common.domain')}
                    value={getDomainAttr(domainInfo, 'domain_name')}
                  />
                  <SettingsField
                    label={t('common.displayName')}
                    value={getDomainAttr(domainInfo, 'domain_display_name')}
                  />
                  <SettingsField label={t('common.status')} value="">
                    <Badge variant="default">
                      {getSystemState(systemStatus) === 'ok' ? t('common.online') : t('common.unavailable')}
                    </Badge>
                  </SettingsField>
                  <SettingsField
                    label={t('settings.kanidmVersion')}
                    value={getSystemProp(systemStatus, 'version')}
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
                Política de Credenciais
              </CardTitle>
              <CardDescription>
                Política de autenticação global (grupo idm_all_persons)
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {policyLoading ? (
                <SettingsFieldsSkeleton count={3} />
              ) : (
                <>
                  <SettingsField label={t('settings.credMin')} value="">
                    <Badge variant={CREDENTIAL_VARIANT[accountPolicy?.credentialTypeMinimum ?? 'any'] ?? 'outline'}>
                      {enumLabel(
                        t,
                        'credentialPolicy',
                        accountPolicy?.credentialTypeMinimum ?? 'any',
                      )}
                    </Badge>
                  </SettingsField>
                  <SettingsField
                    label={t('settings.sessionExpiry')}
                    value={formatSeconds(accountPolicy?.authSessionExpiry)}
                  />
                  <SettingsField
                    label={t('settings.privilegeExpiry')}
                    value={formatSeconds(accountPolicy?.privilegeExpiry)}
                  />
                </>
              )}
            </CardContent>
          </Card>

          <PermissionGate require="system:admin">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Acesso Administrativo</CardTitle>
                <CardDescription>
                  Contas break-glass e recuperação de acesso
                </CardDescription>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">
                  Contas break-glass e configurações avançadas de segurança devem ser
                  gerenciadas via CLI do Kanidm. Consulte a{' '}
                  <a
                    href="https://kanidm.github.io/kanidm/stable/"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-primary underline"
                  >
                    documentação
                    <ExternalLink className="h-3 w-3" />
                  </a>{' '}
                  para detalhes.
                </p>
              </CardContent>
            </Card>
          </PermissionGate>
        </TabsContent>

        <TabsContent value="system" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Informações do Console</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <SettingsField label="ArchGuard Console" value="v1.0.0" />
              <SettingsField label="Framework" value="TanStack Start" />
              <SettingsField
                label={t('settings.kanidmVersion')}
                value={getSystemProp(systemStatus, 'version')}
              />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Backup e Manutenção</CardTitle>
              <CardDescription>
                Operações de backup e restauração do Kanidm
              </CardDescription>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">
                Backups do Kanidm são gerenciados via CLI do servidor
                (<code className="text-xs bg-muted px-1 py-0.5 rounded">kanidmd database backup</code>).
                O Console não gerencia backups diretamente — consulte a{' '}
                <a
                  href="https://kanidm.github.io/kanidm/stable/server_configuration/backup_restore.html"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 text-primary underline"
                >
                  documentação de backup
                  <ExternalLink className="h-3 w-3" />
                </a>.
              </p>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}

// ── Helpers ────────────────────────────────────────

function getDomainAttr(domainInfo: unknown, attr: string): string {
  if (!domainInfo || typeof domainInfo !== 'object') return '—'
  const entry = domainInfo as { attrs?: Record<string, string[]> }
  return entry.attrs?.[attr]?.[0] ?? '—'
}

function getSystemState(status: unknown): string {
  if (!status || typeof status !== 'object') return 'unknown'
  return (status as Record<string, string>).state ?? 'unknown'
}

function getSystemProp(status: unknown, prop: string): string {
  if (!status || typeof status !== 'object') return '—'
  return String((status as Record<string, string>)[prop] ?? '—')
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
