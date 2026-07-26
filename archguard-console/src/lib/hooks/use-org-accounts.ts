import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { OrgAccount, OrgAccountInput } from '@/lib/api/types/org-account'
import {
  approveOrgCheckoutFn,
  checkinOrgAccountFn,
  checkoutOrgAccountFn,
  deleteOrgAccountFn,
  denyOrgCheckoutFn,
  listOrgAccountsFn,
  listPendingOrgCheckoutsFn,
  storeOrgAccountSecretFn,
  upsertOrgAccountFn,
  type CheckoutResult,
} from '@/server/org-accounts-fn'
import type { OrgCheckout } from '@/server/org-checkouts'

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
      rotate?: boolean
      auth_kind?: 'password' | 'api_key' | 'oidc' | 'totp_password'
    }) => storeOrgAccountSecretFn({ data: input }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: orgAccountKeys.all })
    },
  })
}

export const pendingCheckoutKeys = {
  all: ['org-checkouts', 'pending'] as const,
}

export function usePendingOrgCheckouts(enabled = true) {
  return useQuery({
    queryKey: pendingCheckoutKeys.all,
    queryFn: () => listPendingOrgCheckoutsFn() as Promise<OrgCheckout[]>,
    refetchInterval: enabled ? 15_000 : false,
    enabled,
  })
}

export function useApproveOrgCheckout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (checkout_id: string) =>
      approveOrgCheckoutFn({ data: { checkout_id } }) as Promise<CheckoutResult>,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: pendingCheckoutKeys.all })
    },
  })
}

export function useDenyOrgCheckout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: { checkout_id: string; note?: string }) =>
      denyOrgCheckoutFn({ data: input }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: pendingCheckoutKeys.all })
    },
  })
}
