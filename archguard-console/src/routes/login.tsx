// src/routes/login.tsx

import { createFileRoute } from '@tanstack/react-router'
import { Shield } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export const Route = createFileRoute('/login')({
  component: LoginPage,
})

function LoginPage() {
  const handleLogin = async () => {
    const { getUserManager } = await import('@/lib/auth/oidc-config')
    const userManager = getUserManager()
    await userManager.signinRedirect()
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-primary/10">
            <Shield className="h-8 w-8 text-primary" />
          </div>
          <CardTitle className="text-2xl">ArchGuard Console</CardTitle>
          <p className="text-sm text-muted-foreground">
            Gerenciamento de Identidades e Acessos
          </p>
        </CardHeader>
        <CardContent>
          <Button className="w-full" size="lg" onClick={handleLogin}>
            Entrar com ArchGuard ID
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
