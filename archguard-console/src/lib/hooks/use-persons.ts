// src/lib/hooks/use-persons.ts

import {
  useQuery,
  useMutation,
  useQueryClient,
  type UseQueryOptions,
} from '@tanstack/react-query'
import { toast } from 'sonner'
import { personApi } from '@/lib/api/kanidm-client'
import { queryKeys } from '@/lib/utils/query-keys'
import { mapKanidmError } from '@/lib/utils/error-mapper'
import type {
  Person,
  CredentialStatus,
  CreatePersonPayload,
} from '@/lib/api/types/kanidm'

// ── QUERIES ──────────────────────────────────────

export function usePersons(
  options?: Partial<UseQueryOptions<Person[]>>,
) {
  return useQuery({
    queryKey: queryKeys.persons.all,
    queryFn: () => personApi.list(),
    staleTime: 30_000,
    ...options,
  })
}

export function usePerson(
  id: string,
  options?: Partial<UseQueryOptions<Person>>,
) {
  return useQuery({
    queryKey: queryKeys.persons.detail(id),
    queryFn: () => personApi.get(id),
    staleTime: 60_000,
    enabled: !!id,
    ...options,
  })
}

export function usePersonCredentials(
  id: string,
  options?: Partial<UseQueryOptions<CredentialStatus>>,
) {
  return useQuery({
    queryKey: queryKeys.persons.credentials(id),
    queryFn: () => personApi.credentialStatus(id),
    staleTime: 60_000,
    enabled: !!id,
    ...options,
  })
}

// ── MUTATIONS ────────────────────────────────────

export function useCreatePerson() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (payload: CreatePersonPayload) => personApi.create(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.persons.all })
      toast.success('Pessoa criada com sucesso')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useDeletePerson() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => personApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.persons.all })
      toast.success('Pessoa removida com sucesso')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useUpdatePersonAttr() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      id,
      attr,
      values,
    }: {
      id: string
      attr: string
      values: string[]
    }) => personApi.setAttr(id, attr, values),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.persons.detail(variables.id),
      })
      queryClient.invalidateQueries({ queryKey: queryKeys.persons.all })
      toast.success('Atributo atualizado com sucesso')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useAppendPersonAttr() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      id,
      attr,
      values,
    }: {
      id: string
      attr: string
      values: string[]
    }) => personApi.appendAttr(id, attr, values),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.persons.detail(variables.id),
      })
      toast.success('Atributo adicionado com sucesso')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useDeletePersonAttr() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, attr }: { id: string; attr: string }) =>
      personApi.deleteAttr(id, attr),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.persons.detail(variables.id),
      })
      toast.success('Atributo removido com sucesso')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useResetPersonCredential() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, ttl }: { id: string; ttl?: number }) =>
      personApi.createResetToken(id, ttl) as Promise<{ token: string }>,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.persons.credentials(variables.id),
      })
      toast.success('Link de reset gerado com sucesso')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}
