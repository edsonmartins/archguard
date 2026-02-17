// src/components/identity/credential-status.tsx

import { KeySquare, Fingerprint, ShieldCheck, Key, Lock } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { PermissionGate } from '@/components/shared/permission-gate'
import type { CredentialStatus } from '@/lib/api/types/kanidm'

interface CredentialStatusCardProps {
  personId: string
  credentials?: CredentialStatus
  onResetClick: () => void
}

const methodLabel: Record<string, string> = {
  passkey: 'Passkey',
  password_mfa: 'Senha + MFA',
  password_only: 'Apenas Senha',
  none: 'Nenhum',
}

export function CredentialStatusCard({
  credentials,
  onResetClick,
}: CredentialStatusCardProps) {
  if (!credentials) {
    return (
      <Card>
        <CardContent className="py-8 text-center">
          <KeySquare className="mx-auto mb-2 h-8 w-8 text-muted-foreground/50" />
          <p className="text-sm text-muted-foreground">
            Informações de credencial indisponíveis
          </p>
        </CardContent>
      </Card>
    )
  }

  const items = [
    {
      icon: Fingerprint,
      label: 'Passkeys / WebAuthn',
      active: credentials.hasPasskeys || credentials.hasWebauthn,
    },
    {
      icon: Lock,
      label: 'Senha',
      active: credentials.hasPassword,
    },
    {
      icon: ShieldCheck,
      label: 'TOTP (Autenticador)',
      active: credentials.hasTotp,
    },
    {
      icon: Key,
      label: 'Códigos de Backup',
      active: credentials.hasBackupCodes,
    },
  ]

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center justify-between text-base">
            <span>Status de Credenciais</span>
            <Badge
              variant={
                credentials.primaryMethod === 'passkey'
                  ? 'default'
                  : credentials.primaryMethod === 'password_mfa'
                    ? 'default'
                    : credentials.primaryMethod === 'password_only'
                      ? 'secondary'
                      : 'destructive'
              }
            >
              {methodLabel[credentials.primaryMethod] ?? 'Desconhecido'}
            </Badge>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {items.map((item) => (
            <div
              key={item.label}
              className="flex items-center justify-between rounded-lg border p-3"
            >
              <div className="flex items-center gap-3">
                <item.icon className="h-5 w-5 text-muted-foreground" />
                <span className="text-sm font-medium">{item.label}</span>
              </div>
              <Badge
                variant={item.active ? 'default' : 'outline'}
              >
                {item.active ? 'Configurado' : 'Não configurado'}
              </Badge>
            </div>
          ))}
        </CardContent>
      </Card>

      <PermissionGate require="persons:credentials">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Ações de Credencial</CardTitle>
          </CardHeader>
          <CardContent>
            <Button variant="outline" onClick={onResetClick}>
              <KeySquare className="mr-2 h-4 w-4" />
              Gerar Link de Reset
            </Button>
            <p className="mt-2 text-xs text-muted-foreground">
              Gera um link temporário para o usuário redefinir suas credenciais
            </p>
          </CardContent>
        </Card>
      </PermissionGate>
    </div>
  )
}
