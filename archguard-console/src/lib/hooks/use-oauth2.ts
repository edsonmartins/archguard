// src/lib/hooks/use-oauth2.ts

import {
  useQuery,
  useMutation,
  useQueryClient,
  type UseQueryOptions,
} from '@tanstack/react-query'
import { toast } from 'sonner'
import { oauth2Api } from '@/lib/api/kanidm-client'
import { queryKeys } from '@/lib/utils/query-keys'
import { mapKanidmError } from '@/lib/utils/error-mapper'
import type { OAuth2Client, CreateOAuth2ClientPayload } from '@/lib/api/types/kanidm'

// ── QUERIES ──────────────────────────────────────

export function useOAuth2Clients(
  options?: Partial<UseQueryOptions<OAuth2Client[]>>,
) {
  return useQuery({
    queryKey: queryKeys.oauth2.list(),
    queryFn: () => oauth2Api.list(),
    staleTime: 5 * 60_000,
    ...options,
  })
}

export function useOAuth2Client(
  id: string,
  options?: Partial<UseQueryOptions<OAuth2Client>>,
) {
  return useQuery({
    queryKey: queryKeys.oauth2.detail(id),
    queryFn: () => oauth2Api.get(id),
    staleTime: 5 * 60_000,
    enabled: !!id,
    ...options,
  })
}

export function useOAuth2Secret(id: string, enabled = false) {
  return useQuery({
    queryKey: queryKeys.oauth2.secret(id),
    queryFn: () => oauth2Api.getSecret(id) as Promise<string>,
    enabled: !!id && enabled,
    staleTime: Infinity,
  })
}

// ── MUTATIONS ────────────────────────────────────

export function useCreateOAuth2Client() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (payload: CreateOAuth2ClientPayload) =>
      payload.type === 'public'
        ? oauth2Api.createPublic(payload)
        : oauth2Api.createBasic(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.oauth2.all })
      toast.success('Cliente OAuth2 criado com sucesso')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useDeleteOAuth2Client() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => oauth2Api.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.oauth2.all })
      toast.success('Cliente OAuth2 removido com sucesso')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useSetScopeMap() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      clientId,
      groupId,
      scopes,
    }: {
      clientId: string
      groupId: string
      scopes: string[]
    }) => oauth2Api.setScopeMap(clientId, groupId, scopes),
    onSuccess: (_data, { clientId }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.oauth2.detail(clientId),
      })
      toast.success('Scope map atualizado')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useDeleteScopeMap() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      clientId,
      groupId,
    }: {
      clientId: string
      groupId: string
    }) => oauth2Api.deleteScopeMap(clientId, groupId),
    onSuccess: (_data, { clientId }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.oauth2.detail(clientId),
      })
      toast.success('Scope map removido')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useSetSupScopeMap() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      clientId,
      groupId,
      scopes,
    }: {
      clientId: string
      groupId: string
      scopes: string[]
    }) => oauth2Api.setSupScopeMap(clientId, groupId, scopes),
    onSuccess: (_data, { clientId }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.oauth2.detail(clientId),
      })
      toast.success('Sup scope map atualizado')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useSetClaimMap() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      clientId,
      claimName,
      groupId,
      values,
    }: {
      clientId: string
      claimName: string
      groupId: string
      values: string[]
    }) => oauth2Api.setClaimMap(clientId, claimName, groupId, values),
    onSuccess: (_data, { clientId }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.oauth2.detail(clientId),
      })
      toast.success('Claim map atualizado')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useAddRedirectUrl() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ clientId, url }: { clientId: string; url: string }) =>
      oauth2Api.addRedirectUrl(clientId, url),
    onSuccess: (_data, { clientId }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.oauth2.detail(clientId),
      })
      toast.success('URL de redirect adicionada')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useEnableLocalhostRedirects() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (clientId: string) =>
      oauth2Api.enableLocalhostRedirects(clientId),
    onSuccess: (_data, clientId) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.oauth2.detail(clientId),
      })
      toast.success('Redirect localhost habilitado')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function usePreferShortUsername() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (clientId: string) =>
      oauth2Api.preferShortUsername(clientId),
    onSuccess: (_data, clientId) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.oauth2.detail(clientId),
      })
      toast.success('Username curto habilitado')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}
