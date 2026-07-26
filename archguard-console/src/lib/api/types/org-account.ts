/** Org Credential Broker (ADR-013) — metadata only; secrets live in OpenBao. */

export type OrgAccountCategory =
  | 'cloud'
  | 'store'
  | 'product_admin'
  | 'vendor'
  | 'other'

export type OrgAccountCriticality = 'P0' | 'P1' | 'P2'

export type OrgAccountAuthKind =
  | 'password'
  | 'api_key'
  | 'oidc'
  | 'totp_password'

export type OrgAccount = {
  id: string
  slug: string
  name: string
  category: OrgAccountCategory
  product: string
  url: string
  login_hint: string
  auth_kind: OrgAccountAuthKind
  /** OpenBao path e.g. secret/data/org/store/apple-appstore — never the secret value */
  secret_ref: string
  criticality: OrgAccountCriticality
  owner_group: string
  requires_dual_control: boolean
  notes: string
  runbook_url: string
  updated_at: string
  updated_by: string | null
}

export type OrgAccountInput = {
  slug: string
  name: string
  category: OrgAccountCategory
  product?: string
  url?: string
  login_hint?: string
  auth_kind?: OrgAccountAuthKind
  secret_ref?: string
  criticality?: OrgAccountCriticality
  owner_group?: string
  requires_dual_control?: boolean
  notes?: string
  runbook_url?: string
}

export const ORG_CATEGORIES: OrgAccountCategory[] = [
  'cloud',
  'store',
  'product_admin',
  'vendor',
  'other',
]

export const ORG_CRITICALITIES: OrgAccountCriticality[] = ['P0', 'P1', 'P2']

export const ORG_AUTH_KINDS: OrgAccountAuthKind[] = [
  'password',
  'api_key',
  'oidc',
  'totp_password',
]
