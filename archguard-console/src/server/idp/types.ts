// Identity-provider admin port.
//
// The console does not only authenticate against the IdP — it writes to it:
// ensures tenant groups, binds members and disables people on offboarding.
// That surface was hardcoded against Kanidm's REST API. ArchGuard is now a
// Casdoor fork with a different API, so the write path lives behind this port
// and each IdP gets an adapter.
//
// Authentication (OIDC) is separate and stays in auth.ts / operator-session.ts.

export type EnsureGroupAction = 'created' | 'exists' | 'skipped' | 'error'

export type EnsureGroupResult = {
  name: string
  action: EnsureGroupAction
  error?: string
}

export type AdminStep = {
  ok: boolean
  /** Human-readable outcome, surfaced in lifecycle/offboarding evidence. */
  detail: string
}

export interface IdentityAdmin {
  /** Which IdP this adapter talks to; shown in /platform diagnostics. */
  readonly kind: 'kanidm' | 'archguard'

  /** True when the adapter has the URL and credential it needs. */
  configured(): boolean

  /** Idempotent group creation. Must not fail when the group already exists. */
  ensureGroup(name: string, description?: string): Promise<EnsureGroupResult>

  /** Idempotent membership bind. Must not fail when already a member. */
  addUserToGroup(username: string, group: string): Promise<AdminStep>

  /**
   * Block sign-in for a principal without deleting it — the audit trail must
   * keep pointing at a real subject after offboarding.
   */
  disableUser(username: string): Promise<AdminStep>
}
