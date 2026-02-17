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
} from 'lucide-react'
import { Link } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
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
import { usePerson, useDeletePerson, usePersonCredentials } from '@/lib/hooks/use-persons'
import { initials } from '@/lib/utils/formatters'

export function PersonDetailPage() {
  const { personId } = Route.useParams()
  const navigate = useNavigate()
  const { data: person, isLoading } = usePerson(personId)
  const { data: credentials } = usePersonCredentials(personId)
  const deletePerson = useDeletePerson()
  const [showDelete, setShowDelete] = useState(false)
  const [showReset, setShowReset] = useState(false)

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
        <div className="flex gap-2">
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
                  <CopyButton text={person.username} />
                </InfoRow>
                <InfoRow label="Nome de Exibição" value={person.displayName} />
                {person.legalName && (
                  <InfoRow label="Nome Legal" value={person.legalName} />
                )}
                <InfoRow label="ID" value={person.id}>
                  <CopyButton text={person.id} />
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
                      <CopyButton text={email} />
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

      <ConfirmDialog
        open={showDelete}
        onOpenChange={setShowDelete}
        title="Excluir Pessoa"
        description={`Esta ação é irreversível. Todos os dados de ${person.displayName} serão permanentemente removidos.`}
        confirmText={person.username}
        destructive
        isLoading={deletePerson.isPending}
        onConfirm={() => {
          deletePerson.mutate(person.id, {
            onSuccess: () => {
              navigate({ to: '/identities' })
            },
          })
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
