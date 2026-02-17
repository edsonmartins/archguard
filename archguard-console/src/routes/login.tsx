// src/routes/login.tsx

import { useState } from 'react'
import { createFileRoute } from '@tanstack/react-router'
import { Shield, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'

export const Route = createFileRoute('/login')({
  component: LoginPage,
})

function LoginPage() {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleLogin = async () => {
    setLoading(true)
    setError(null)

    try {
      const { getUserManager } = await import('@/lib/auth/oidc-config')
      const userManager = getUserManager()
      await userManager.signinRedirect()
    } catch (err) {
      console.error('Login redirect error:', err)
      const message =
        err instanceof Error ? err.message : 'Erro desconhecido'

      if (message.includes('fetch') || message.includes('Network')) {
        setError(
          'Não foi possível conectar ao servidor de identidade. Verifique se o Kanidm está acessível.',
        )
      } else {
        setError(`Erro ao iniciar login: ${message}`)
      }
      setLoading(false)
    }
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
        <CardContent className="space-y-4">
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          <Button
            className="w-full"
            size="lg"
            onClick={handleLogin}
            disabled={loading}
          >
            {loading ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Conectando...
              </>
            ) : (
              'Entrar com ArchGuard ID'
            )}
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
