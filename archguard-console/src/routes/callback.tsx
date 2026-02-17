// src/routes/callback.tsx

import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { loginCallbackFn } from '@/server/auth'

export const Route = createFileRoute('/callback')({
  component: CallbackPage,
})

function CallbackPage() {
  const navigate = useNavigate()
  const processed = useRef(false)

  useEffect(() => {
    if (processed.current) return
    processed.current = true

    handleCallback()

    async function handleCallback() {
      try {
        const url = new URL(window.location.href)
        const code = url.searchParams.get('code')
        const state = url.searchParams.get('state')

        if (!code || !state) {
          throw new Error('Missing code or state in callback URL')
        }

        // Extract code_verifier from oidc-client-ts sessionStorage
        // oidc-client-ts stores state as: oidc.{state} -> JSON with code_verifier
        const storedStateKey = `oidc.${state}`
        const storedStateRaw = sessionStorage.getItem(storedStateKey)
        let codeVerifier = ''

        if (storedStateRaw) {
          try {
            const storedState = JSON.parse(storedStateRaw)
            codeVerifier = storedState.code_verifier || ''
          } catch {
            // Fallback: try to find any oidc state entry with code_verifier
          }
        }

        // Clean up oidc-client-ts state from sessionStorage
        for (let i = sessionStorage.length - 1; i >= 0; i--) {
          const key = sessionStorage.key(i)
          if (key?.startsWith('oidc.')) {
            sessionStorage.removeItem(key)
          }
        }

        // Exchange code for session on server (server does the token exchange)
        const result = await loginCallbackFn({
          data: {
            code,
            state,
            codeVerifier,
            redirectUri: `${window.location.origin}/callback`,
          },
        })

        if (result.success) {
          navigate({ to: result.redirect || '/dashboard' })
        } else if (result.error === 'unauthorized') {
          navigate({ to: '/unauthorized' })
        } else {
          navigate({ to: '/login' })
        }
      } catch (error) {
        console.error('OIDC callback error:', error)
        navigate({ to: '/login' })
      }
    }
  }, [navigate])

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="text-center">
        <div className="mx-auto mb-4 h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
        <p className="text-muted-foreground">Autenticando...</p>
      </div>
    </div>
  )
}
