// src/routes/callback.tsx

import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { createSessionFromTokensFn } from '@/server/auth'

export const Route = createFileRoute('/callback')({
  component: CallbackPage,
})

function CallbackPage() {
  const navigate = useNavigate()
  const processed = useRef(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (processed.current) return
    processed.current = true

    handleCallback()

    async function handleCallback() {
      try {
        const url = new URL(window.location.href)

        // Handle OAuth2 error response from Kanidm
        const errorParam = url.searchParams.get('error')
        const errorDesc = url.searchParams.get('error_description')
        if (errorParam) {
          setError(`Kanidm: ${errorParam} — ${errorDesc || 'Erro desconhecido'}`)
          return
        }

        // Use oidc-client-ts signinCallback to handle the full OIDC flow
        // This manages state/code_verifier/token exchange internally
        const { getUserManager } = await import('@/lib/auth/oidc-config')
        const userManager = getUserManager()
        const user = await userManager.signinCallback(window.location.href)

        if (!user || !user.id_token) {
          setError('OIDC signinCallback não retornou um usuário válido.')
          return
        }

        // Send tokens to server to create the session cookie
        const result = await createSessionFromTokensFn({
          data: {
            accessToken: user.access_token,
            idToken: user.id_token,
            refreshToken: user.refresh_token || undefined,
            expiresIn: user.expires_in ?? 3600,
          },
        })

        if (result.success) {
          navigate({ to: result.redirect || '/dashboard' })
        } else if (result.error === 'unauthorized') {
          navigate({ to: '/unauthorized' })
        } else {
          setError(result.error || 'Erro desconhecido no servidor')
        }
      } catch (err) {
        console.error('OIDC callback error:', err)
        const message = err instanceof Error ? err.message : String(err)

        if (message.includes('fetch') || message.includes('NetworkError') || message.includes('Failed to fetch')) {
          setError(
            `Erro de rede ao trocar código OAuth2. Verifique se o certificado TLS do Kanidm foi aceito no navegador. ` +
            `Acesse https://localhost:8443 e aceite o certificado, depois tente novamente. (${message})`
          )
        } else if (message.includes('No matching state')) {
          setError(
            'Estado OIDC não encontrado. Isso pode acontecer se a sessão expirou. Tente fazer login novamente.'
          )
        } else {
          setError(`Erro no callback OIDC: ${message}`)
        }
      }
    }
  }, [navigate])

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="mx-auto max-w-lg rounded-lg border border-destructive/50 bg-destructive/10 p-6 text-center">
          <p className="mb-2 text-lg font-semibold text-destructive">Erro de Autenticação</p>
          <p className="mb-4 text-sm text-muted-foreground">{error}</p>
          <button
            onClick={() => {
              // Clear any stale OIDC state
              for (let i = sessionStorage.length - 1; i >= 0; i--) {
                const key = sessionStorage.key(i)
                if (key?.startsWith('oidc.')) {
                  sessionStorage.removeItem(key)
                }
              }
              window.location.href = '/login'
            }}
            className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground hover:bg-primary/90"
          >
            Tentar novamente
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="text-center">
        <div className="mx-auto mb-4 h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
        <p className="text-muted-foreground">Autenticando...</p>
      </div>
    </div>
  )
}
