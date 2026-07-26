import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { OrgAccount, OrgAccountInput } from '@/lib/api/types/org-account'
import {
  deleteOrgAccountFn,
  listOrgAccountsFn,
  upsertOrgAccountFn,
} from '@/server/org-accounts-fn'

export const orgAccountKeys = {
  all: ['org-accounts'] as const,
}

export function useOrgAccounts() {
  return useQuery({
    queryKey: orgAccountKeys.all,
    queryFn: () => listOrgAccountsFn() as Promise<OrgAccount[]>,
  })
}

export function useUpsertOrgAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: OrgAccountInput) =>
      upsertOrgAccountFn({ data: input }) as Promise<OrgAccount>,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: orgAccountKeys.all })
    },
  })
}

export function useDeleteOrgAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      deleteOrgAccountFn({ data: { id } }) as Promise<{ ok: boolean }>,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: orgAccountKeys.all })
    },
  })
}
