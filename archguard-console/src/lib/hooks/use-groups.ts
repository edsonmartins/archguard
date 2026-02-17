// src/lib/hooks/use-groups.ts

import {
  useQuery,
  useMutation,
  useQueryClient,
  type UseQueryOptions,
} from '@tanstack/react-query'
import { toast } from 'sonner'
import { groupApi } from '@/lib/api/kanidm-client'
import { queryKeys } from '@/lib/utils/query-keys'
import { mapKanidmError } from '@/lib/utils/error-mapper'
import type { Group, CreateGroupPayload } from '@/lib/api/types/kanidm'

// ── QUERIES ──────────────────────────────────────

export function useGroups(
  options?: Partial<UseQueryOptions<Group[]>>,
) {
  return useQuery({
    queryKey: queryKeys.groups.all,
    queryFn: () => groupApi.list(),
    staleTime: 60_000,
    ...options,
  })
}

export function useGroup(
  id: string,
  options?: Partial<UseQueryOptions<Group>>,
) {
  return useQuery({
    queryKey: queryKeys.groups.detail(id),
    queryFn: () => groupApi.get(id),
    staleTime: 60_000,
    enabled: !!id,
    ...options,
  })
}

export function useGroupMembers(id: string) {
  return useQuery({
    queryKey: queryKeys.groups.members(id),
    queryFn: () => groupApi.getMembers(id) as Promise<string[]>,
    staleTime: 60_000,
    enabled: !!id,
  })
}

// ── MUTATIONS ────────────────────────────────────

export function useCreateGroup() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (payload: CreateGroupPayload) => groupApi.create(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.groups.all })
      toast.success('Grupo criado com sucesso')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useDeleteGroup() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => groupApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.groups.all })
      toast.success('Grupo removido com sucesso')
    },
    onError: (error) => {
      toast.error(mapKanidmError(error))
    },
  })
}

export function useAddGroupMembers() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, memberIds }: { id: string; memberIds: string[] }) =>
      groupApi.addMembers(id, memberIds),
    onMutate: async ({ id, memberIds }) => {
      await queryClient.cancelQueries({
        queryKey: queryKeys.groups.detail(id),
      })
      const previousGroup = queryClient.getQueryData<Group>(
        queryKeys.groups.detail(id),
      )
      if (previousGroup) {
        queryClient.setQueryData<Group>(queryKeys.groups.detail(id), {
          ...previousGroup,
          memberCount: previousGroup.memberCount + memberIds.length,
        })
      }
      return { previousGroup }
    },
    onError: (error, { id }, context) => {
      if (context?.previousGroup) {
        queryClient.setQueryData(
          queryKeys.groups.detail(id),
          context.previousGroup,
        )
      }
      toast.error(mapKanidmError(error))
    },
    onSettled: (_data, _error, { id }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.groups.detail(id),
      })
      queryClient.invalidateQueries({
        queryKey: queryKeys.groups.members(id),
      })
      queryClient.invalidateQueries({ queryKey: queryKeys.persons.all })
    },
    onSuccess: () => {
      toast.success('Membros adicionados com sucesso')
    },
  })
}

export function useRemoveGroupMembers() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, memberIds }: { id: string; memberIds: string[] }) =>
      groupApi.removeMembers(id, memberIds),
    onMutate: async ({ id, memberIds }) => {
      await queryClient.cancelQueries({
        queryKey: queryKeys.groups.detail(id),
      })
      const previousGroup = queryClient.getQueryData<Group>(
        queryKeys.groups.detail(id),
      )
      if (previousGroup) {
        queryClient.setQueryData<Group>(queryKeys.groups.detail(id), {
          ...previousGroup,
          members: previousGroup.members.filter(
            (m) => !memberIds.includes(m.id),
          ),
          memberCount: Math.max(
            0,
            previousGroup.memberCount - memberIds.length,
          ),
        })
      }
      return { previousGroup }
    },
    onError: (error, { id }, context) => {
      if (context?.previousGroup) {
        queryClient.setQueryData(
          queryKeys.groups.detail(id),
          context.previousGroup,
        )
      }
      toast.error(mapKanidmError(error))
    },
    onSettled: (_data, _error, { id }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.groups.detail(id),
      })
      queryClient.invalidateQueries({
        queryKey: queryKeys.groups.members(id),
      })
      queryClient.invalidateQueries({ queryKey: queryKeys.persons.all })
    },
    onSuccess: () => {
      toast.success('Membros removidos com sucesso')
    },
  })
}
