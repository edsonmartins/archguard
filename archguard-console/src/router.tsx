// src/router.tsx

import { createRouter as createTanStackRouter } from '@tanstack/react-router'
import { QueryClient, MutationCache, QueryCache } from '@tanstack/react-query'
import { toast } from 'sonner'
import { routeTree } from './routeTree.gen'
import { reportError } from '@/lib/utils/error-sink'

export function getRouter() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        gcTime: 5 * 60_000,
        retry: (failureCount, error) => {
          // Don't retry auth errors
          if (error instanceof Error && (error.message.includes('401') || error.message.includes('403'))) {
            return false
          }
          return failureCount < 1
        },
        refetchOnWindowFocus: true,
      },
    },
    queryCache: new QueryCache({
      onError: (error) => {
        if (error instanceof Error) {
          if (error.message.includes('401')) {
            // Session expired — handled by route guard
            return
          }
          if (error.message.includes('403')) {
            toast.error('Sem permissão para esta operação')
            return
          }
          if (error.message.includes('fetch') || error.message.includes('network')) {
            toast.error('Erro de conexão com o servidor')
            return
          }
        }
      },
    }),
    mutationCache: new MutationCache({
      onError: (error) => {
        // Individual mutations handle their own errors via hooks; this is
        // a fallback for unhandled mutation errors.
        reportError(error, { source: 'mutation-cache' })
      },
    }),
  })

  const router = createTanStackRouter({
    routeTree,
    scrollRestoration: true,
    defaultPreload: 'intent',
    defaultPreloadStaleTime: 0,
    context: {
      queryClient,
    },
  })

  return router
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof getRouter>
  }
}
