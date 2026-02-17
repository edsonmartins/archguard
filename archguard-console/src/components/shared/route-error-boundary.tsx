// src/components/shared/route-error-boundary.tsx

import { useRouter } from '@tanstack/react-router'
import { AlertTriangle, RefreshCw, Home } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

interface RouteErrorBoundaryProps {
  error: Error
  reset?: () => void
}

export function RouteErrorBoundary({ error, reset }: RouteErrorBoundaryProps) {
  const router = useRouter()

  const is401 = error.message.includes('401')
  const is403 = error.message.includes('403')
  const isNetwork =
    error.message.includes('fetch') ||
    error.message.includes('network') ||
    error.message.includes('Failed to fetch')

  if (is401) {
    router.navigate({ to: '/login' })
    return null
  }

  return (
    <div className="flex min-h-[50vh] items-center justify-center p-8">
      <Card className="max-w-md">
        <CardContent className="pt-6 text-center">
          <AlertTriangle className="mx-auto mb-4 h-12 w-12 text-destructive" />
          <h2 className="mb-2 text-lg font-semibold">
            {is403
              ? 'Acesso Negado'
              : isNetwork
                ? 'Erro de Conexão'
                : 'Algo deu errado'}
          </h2>
          <p className="mb-4 text-sm text-muted-foreground">
            {is403
              ? 'Você não tem permissão para acessar este recurso.'
              : isNetwork
                ? 'Não foi possível conectar ao servidor. Verifique sua conexão.'
                : error.message || 'Ocorreu um erro inesperado.'}
          </p>
          <div className="flex justify-center gap-2">
            {reset && (
              <Button variant="outline" onClick={reset}>
                <RefreshCw className="mr-2 h-4 w-4" />
                Tentar Novamente
              </Button>
            )}
            <Button
              variant="outline"
              onClick={() => router.navigate({ to: '/dashboard' })}
            >
              <Home className="mr-2 h-4 w-4" />
              Dashboard
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
