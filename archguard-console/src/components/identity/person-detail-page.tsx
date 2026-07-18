import { useTranslation } from 'react-i18next'
// src/components/identity/person-detail-page.tsx

import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { Route } from '@/routes/_authed/identities/$personId'
import {
  ArrowLeft,
  Mail,
  Shield,
  KeySquare,
  Users,
  History,
  Trash2,
  UserX,
  CheckCircle2,
  Circle,
  Loader2,
} from 'lucide-react'
import { Link } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge } from '@/components/shared/status-badge'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { PermissionGate } from '@/components/shared/permission-gate'
import { CopyButton } from '@/components/shared/copy-button'
import { TimeAgo } from '@/components/shared/time-ago'
import { GroupBadge } from '@/components/shared/group-badge'
import { CredentialStatusCard } from '@/components/identity/credential-status'
import { CredentialResetDialog } from '@/components/identity/credential-reset-dialog'
import { PersonGroupAssignment } from '@/components/identity/group-assignment'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { usePerson, useDeletePerson, usePersonCredentials } from '@/lib/hooks/use-persons'
import { queryKeys } from '@/lib/utils/query-keys'
import { initials } from '@/lib/utils/formatters'
import {
  revokePersonAccessFn,
  type OffboardStep,
} from '@/server/offboarding-fn'
import {
  grantPersonTargetFn,
  provisionPersonAccessFn,
  type LifecycleStep,
} from '@/server/lifecycle-fn'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

export function PersonDetailPage() {
  const { t } = useTranslation()
  const { personId } = Route.useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { data: person, isLoading } = usePerson(personId)
  const { data: credentials } = usePersonCredentials(personId)
  const deletePerson = useDeletePerson()
  const [showDelete, setShowDelete] = useState(false)
  const [showReset, setShowReset] = useState(false)
  const [showRevoke, setShowRevoke] = useState(false)
  const [showProvision, setShowProvision] = useState(false)
  const [showGrant, setShowGrant] = useState(false)
  const [revokeSteps, setRevokeSteps] = useState<OffboardStep[] | null>(null)
  const [lifecycleSteps, setLifecycleSteps] = useState<LifecycleStep[] | null>(
    null,
  )
  const [tenantSlug, setTenantSlug] = useState('')
  const [grantTarget, setGrantTarget] = useState('')
  const [grantTtl, setGrantTtl] = useState('8h')

  const revokeAccess = useMutation({
    mutationFn: () =>
      revokePersonAccessFn({
        data: {
          username: person!.username,
          person_id: personId,
          reason: `console offboarding by admin`,
        },
      }),
    onSuccess: (res) => {
      setRevokeSteps(res.steps)
      setShowRevoke(false)
      void queryClient.invalidateQueries({
        queryKey: queryKeys.persons.detail(personId),
      })
      void queryClient.invalidateQueries({ queryKey: queryKeys.persons.all })
      if (res.ok) toast.success(res.message)
      else toast.error(res.message)
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const provision = useMutation({
    mutationFn: () => {
      const tenant =
        tenantSlug.trim() ||
        person!.groupNames.find((g) => g.startsWith('tenant_'))?.replace(
          /^tenant_/,
          '',
        ) ||
        'default'
      return provisionPersonAccessFn({
        data: {
          username: person!.username,
          email: person!.emails?.[0] || '',
          tenant_slug: tenant,
          groups: person!.groupNames || [],
          profile: 'operator',
        },
      })
    },
    onSuccess: (res) => {
      setLifecycleSteps(res.steps)
      setShowProvision(false)
      void queryClient.invalidateQueries({
        queryKey: queryKeys.persons.detail(personId),
      })
      if (res.ok) toast.success(res.message)
      else toast.error(res.message)
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const grant = useMutation({
    mutationFn: () =>
      grantPersonTargetFn({
        data: {
          username: person!.username,
          target: grantTarget.trim(),
          ttl: grantTtl.trim() || '8h',
        },
      }),
    onSuccess: (res) => {
      setLifecycleSteps(res.steps)
      setShowGrant(false)
      if (res.ok) toast.success(res.message)
      else toast.error(res.message)
    },
    onError: (e) => toast.error((e as Error).message),
  })

  if (isLoading || !person) {
    return <PersonDetailSkeleton />
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/identities">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <Avatar className="h-12 w-12">
          <AvatarFallback>{initials(person.displayName)}</AvatarFallback>
        </Avatar>
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <h1 className="text-2xl font-bold">{person.displayName}</h1>
            <StatusBadge status={person.status} />
          </div>
          <p className="text-muted-foreground">@{person.username}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <PermissionGate require={['persons:update', 'persons:create']} any>
            <Button
              variant="outline"
              onClick={() => {
                const t0 =
                  person.groupNames
                    .find((g) => g.startsWith('tenant_'))
                    ?.replace(/^tenant_/, '') || ''
                setTenantSlug(t0)
                setShowProvision(true)
              }}
              disabled={provision.isPending}
            >
              {provision.isPending ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <Shield className="mr-2 h-4 w-4" />
              )}
              Provisionar acesso
            </Button>
          </PermissionGate>
          <PermissionGate require={['persons:update', 'gateways:manage']} any>
            <Button
              variant="outline"
              onClick={() => setShowGrant(true)}
              disabled={grant.isPending}
            >
              Grant target
            </Button>
          </PermissionGate>
          <PermissionGate require={['persons:update', 'persons:delete']} any>
            <Button
              variant="secondary"
              onClick={() => setShowRevoke(true)}
              disabled={revokeAccess.isPending}
            >
              {revokeAccess.isPending ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <UserX className="mr-2 h-4 w-4" />
              )}
              Revogar acesso
            </Button>
          </PermissionGate>
          <PermissionGate require="persons:credentials">
            <Button variant="outline" onClick={() => setShowReset(true)}>
              <KeySquare className="mr-2 h-4 w-4" />
              Reset Credencial
            </Button>
          </PermissionGate>
          <PermissionGate require="persons:delete">
            <Button
              variant="destructive"
              onClick={() => setShowDelete(true)}
            >
              <Trash2 className="mr-2 h-4 w-4" />
              Excluir
            </Button>
          </PermissionGate>
        </div>
      </div>

      {(revokeSteps || lifecycleSteps) && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">
              {revokeSteps ? 'Último offboarding' : 'Último provision/grant'}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ul className="space-y-2 text-sm">
              {(revokeSteps || lifecycleSteps || []).map((s, i) => (
                <li key={`${s.component}-${i}`} className="flex items-start gap-2">
                  {s.ok ? (
                    <CheckCircle2 className="h-4 w-4 text-emerald-600 shrink-0 mt-0.5" />
                  ) : (
                    <Circle className="h-4 w-4 text-amber-600 shrink-0 mt-0.5" />
                  )}
                  <span>
                    <span className="font-mono text-xs">{s.component}</span>
                    {s.detail ? (
                      <span className="text-muted-foreground"> — {s.detail}</span>
                    ) : null}
                  </span>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      )}

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">
            <Shield className="mr-2 h-4 w-4" />
            Visão Geral
          </TabsTrigger>
          <TabsTrigger value="groups">
            <Users className="mr-2 h-4 w-4" />
            Grupos
          </TabsTrigger>
          <TabsTrigger value="credentials">
            <KeySquare className="mr-2 h-4 w-4" />
            Credenciais
          </TabsTrigger>
          <TabsTrigger value="history">
            <History className="mr-2 h-4 w-4" />
            Histórico
          </TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Informações</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <InfoRow label="Username" value={`@${person.username}`}>
                  <CopyButton value={person.username} />
                </InfoRow>
                <InfoRow label={t('identities.displayName')} value={person.displayName} />
                {person.legalName && (
                  <InfoRow label="Nome Legal" value={person.legalName} />
                )}
                <InfoRow label="ID" value={person.id}>
                  <CopyButton value={person.id} />
                </InfoRow>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">Contato</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                {person.emails.length > 0 ? (
                  person.emails.map((email) => (
                    <div
                      key={email}
                      className="flex items-center gap-2 text-sm"
                    >
                      <Mail className="h-4 w-4 text-muted-foreground" />
                      <span>{email}</span>
                      <CopyButton value={email} />
                    </div>
                  ))
                ) : (
                  <p className="text-sm text-muted-foreground">
                    Nenhum email cadastrado
                  </p>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">Segurança</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <InfoRow label="Status" value="">
                  <StatusBadge status={person.status} />
                </InfoRow>
                <InfoRow
                  label="MFA"
                  value={
                    credentials?.primaryMethod === 'passkey'
                      ? 'Passkey'
                      : credentials?.primaryMethod === 'password_mfa'
                        ? 'Senha + MFA'
                        : credentials?.primaryMethod === 'password_only'
                          ? 'Apenas Senha'
                          : 'Nenhum'
                  }
                />
                {person.accountExpiry && (
                  <InfoRow label="Expira em" value="">
                    <TimeAgo date={person.accountExpiry} />
                  </InfoRow>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">Grupos</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex flex-wrap gap-1">
                  {person.groupNames.length > 0 ? (
                    person.groupNames.map((g) => (
                      <GroupBadge key={g} name={g} />
                    ))
                  ) : (
                    <p className="text-sm text-muted-foreground">
                      Nenhum grupo
                    </p>
                  )}
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="groups">
          <PersonGroupAssignment personId={personId} person={person} />
        </TabsContent>

        <TabsContent value="credentials">
          <CredentialStatusCard
            personId={personId}
            credentials={credentials}
            onResetClick={() => setShowReset(true)}
          />
        </TabsContent>

        <TabsContent value="history">
          <Card>
            <CardContent className="py-8 text-center">
              <History className="mx-auto mb-2 h-8 w-8 text-muted-foreground/50" />
              <p className="text-sm text-muted-foreground">
                Histórico de auditoria será implementado na Fase 3
              </p>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      <Dialog open={showProvision} onOpenChange={setShowProvision}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Provisionar acesso</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Garante grupos tenant + archguard_users no Kanidm e dispara
            orquestração (adapters). Pessoa já deve existir no IdP.
          </p>
          <div className="space-y-2">
            <Label>Tenant slug (sem prefixo tenant_)</Label>
            <Input
              className="font-mono"
              value={tenantSlug}
              onChange={(e) => setTenantSlug(e.target.value)}
              placeholder="rio_quality"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowProvision(false)}>
              Cancelar
            </Button>
            <Button
              disabled={provision.isPending || !tenantSlug.trim()}
              onClick={() => provision.mutate()}
            >
              {provision.isPending && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              Provisionar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={showGrant} onOpenChange={setShowGrant}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Grant target</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Concede acesso a um target/role via orquestração (Warpgate/OpenBao).
          </p>
          <div className="space-y-2">
            <Label>Target ou role</Label>
            <Input
              className="font-mono"
              value={grantTarget}
              onChange={(e) => setGrantTarget(e.target.value)}
              placeholder="rio-api-1"
            />
          </div>
          <div className="space-y-2">
            <Label>TTL</Label>
            <Input
              className="font-mono"
              value={grantTtl}
              onChange={(e) => setGrantTtl(e.target.value)}
              placeholder="8h"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowGrant(false)}>
              Cancelar
            </Button>
            <Button
              disabled={grant.isPending || !grantTarget.trim()}
              onClick={() => grant.mutate()}
            >
              {grant.isPending && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              Conceder
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={showRevoke}
        onOpenChange={setShowRevoke}
        title="Revogar acesso"
        description={`Bloqueia o login de ${person.displayName} (@${person.username}) no Kanidm, aciona orquestração (WG/OpenBao) e remove usuário no Warpgate se existir. Preferível a excluir (mantém trilha de auditoria). Digite o username para confirmar.`}
        confirmText={person.username}
        destructive
        isLoading={revokeAccess.isPending}
        onConfirm={() => {
          revokeAccess.mutate()
        }}
      />

      <ConfirmDialog
        open={showDelete}
        onOpenChange={setShowDelete}
        title={t('identities.deletePerson')}
        description={`Esta ação é irreversível. Todos os dados de ${person.displayName} serão permanentemente removidos.`}
        confirmText={person.username}
        destructive
        isLoading={deletePerson.isPending}
        onConfirm={async () => {
          await deletePerson.mutateAsync(person.id)
          // Force /identities to do a cold fetch on mount so the removed
          // entry is gone on the first paint, not after a hydrate flash.
          queryClient.removeQueries({ queryKey: queryKeys.persons.all })
          navigate({ to: '/identities' })
        }}
      />

      <CredentialResetDialog
        open={showReset}
        onOpenChange={setShowReset}
        personId={personId}
        personName={person.displayName}
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

function PersonDetailSkeleton() {
  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Skeleton className="h-10 w-10" />
        <Skeleton className="h-12 w-12 rounded-full" />
        <div className="flex-1">
          <Skeleton className="h-8 w-48" />
          <Skeleton className="mt-1 h-4 w-24" />
        </div>
      </div>
      <Skeleton className="h-10 w-96" />
      <div className="grid gap-4 md:grid-cols-2">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-48" />
        ))}
      </div>
    </div>
  )
}
