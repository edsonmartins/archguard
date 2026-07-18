import { useTranslation } from 'react-i18next'
// src/routes/unauthorized.tsx

import { createFileRoute, Link } from '@tanstack/react-router'
import { ShieldAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export const Route = createFileRoute('/unauthorized')({
  component: UnauthorizedPage,
})

function UnauthorizedPage() {
  const { t } = useTranslation()
  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <Card className="w-full max-w-md text-center">
        <CardHeader>
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-destructive/10">
            <ShieldAlert className="h-8 w-8 text-destructive" />
          </div>
          <CardTitle className="text-2xl">Acesso Negado</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground">
            Sua conta não possui permissões de administrador para acessar o
            ArchGuard Console.
          </p>
          <p className="text-sm text-muted-foreground">
            Entre em contato com o administrador do sistema para solicitar
            acesso.
          </p>
          <Button asChild variant="outline">
            <Link to="/login">Voltar ao Login</Link>
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
