import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { OrgAccount, OrgAccountInput } from '@/lib/api/types/org-account'
import {
  checkinOrgAccountFn,
  checkoutOrgAccountFn,
  deleteOrgAccountFn,
  listOrgAccountsFn,
  storeOrgAccountSecretFn,
  upsertOrgAccountFn,
  type CheckoutResult,
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

export function useCheckoutOrgAccount() {
  return useMutation({
    mutationFn: (input: {
      id: string
      reason: string
      ttl_seconds?: number
    }) => checkoutOrgAccountFn({ data: input }) as Promise<CheckoutResult>,
  })
}

export function useCheckinOrgAccount() {
  return useMutation({
    mutationFn: (checkout_id: string) =>
      checkinOrgAccountFn({ data: { checkout_id } }),
  })
}

export function useStoreOrgAccountSecret() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: {
      id: string
      password?: string
      username?: string
      api_key?: string
      note?: string
    }) => storeOrgAccountSecretFn({ data: input }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: orgAccountKeys.all })
    },
  })
}
