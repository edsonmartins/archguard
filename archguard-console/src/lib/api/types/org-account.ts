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

/**
 * OCB-4 — how far the product/cloud moved off shared passwords.
 * - password_only: shared password is primary (bad)
 * - oidc_primary: day-to-day via Kanidm/Workspace SSO; password only break-glass in OpenBao
 * - oidc_only: no shared password (ideal)
 * - api_key_primary: automation via key in OpenBao; human via individual SSO
 * - external_idp: Google/Apple native IAM (not Kanidm) but no shared password
 */
export type OrgFederationStatus =
  | 'password_only'
  | 'oidc_primary'
  | 'oidc_only'
  | 'api_key_primary'
  | 'external_idp'

export type OrgAccount = {
  id: string
  slug: string
  name: string
  category: OrgAccountCategory
  product: string
  url: string
  login_hint: string
  auth_kind: OrgAccountAuthKind
  federation_status: OrgFederationStatus
  /** Kanidm OAuth2 client id when federated, e.g. vendax-admin */
  oidc_client_id: string
  /** OpenBao path e.g. secret/data/org/store/apple-appstore — never the secret value */
  secret_ref: string
  criticality: OrgAccountCriticality
  owner_group: string
  requires_dual_control: boolean
  notes: string
  runbook_url: string
  /** ISO time of last secret rotation in OpenBao (metadata only). */
  rotated_at: string | null
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
  federation_status?: OrgFederationStatus
  oidc_client_id?: string
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

export const ORG_FEDERATION_STATUSES: OrgFederationStatus[] = [
  'password_only',
  'oidc_primary',
  'oidc_only',
  'api_key_primary',
  'external_idp',
]
