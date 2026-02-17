// src/lib/hooks/use-service-accounts.ts

import {
  useQuery,
  useMutation,
  useQueryClient,
  type UseQueryOptions,
} from '@tanstack/react-query'
import { toast } from 'sonner'
import { serviceAccountApi } from '@/lib/api/kanidm-client'
import { queryKeys } from '@/lib/utils/query-keys'
import { mapKanidmError } from '@/lib/utils/error-mapper'
import type {
  ServiceAccount,
  CreateServiceAccountPayload,
} from '@/lib/api/types/kanidm'

// ── QUERIES ──────────────────────────────────────

export function useServiceAccounts(
  options?: Partial<UseQueryOptions<ServiceAccount[]>>,
) {
  return useQuery({
    queryKey: queryKeys.serviceAccounts.list(),
    queryFn: () => serviceAccountApi.list(),
    staleTime: 60_000,
    ...options,
  })
}

export function useServiceAccount(
  id: string,
  options?: Partial<UseQueryOptions<ServiceAccount>>,
) {
  return useQuery({
    queryKey: queryKeys.serviceAccounts.detail(id),
    queryFn: () => serviceAccountApi.get(id),
    staleTime: 60_000,
    enabled: !!id,
    ...options,
  })
}

// ── MUTATIONS ────────────────────────────────────

export function useCreateServiceAccount() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (payload: CreateServiceAccountPayload) =>
      serviceAccountApi.create(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.serviceAccounts.all,
      })
      toast.success('Service account criado com sucesso')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useDeleteServiceAccount() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => serviceAccountApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.serviceAccounts.all,
      })
      toast.success('Service account removido com sucesso')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useGenerateApiToken() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      id,
      label,
      expiry,
    }: {
      id: string
      label: string
      expiry?: string
    }) => serviceAccountApi.generateToken(id, label, expiry) as Promise<{ token: string }>,
    onSuccess: (_data, { id }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.serviceAccounts.detail(id),
      })
      toast.success('Token API gerado com sucesso')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useRevokeApiToken() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, tokenId }: { id: string; tokenId: string }) =>
      serviceAccountApi.revokeToken(id, tokenId),
    onSuccess: (_data, { id }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.serviceAccounts.detail(id),
      })
      toast.success('Token API revogado com sucesso')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}
