// src/lib/api/types/kanidm.ts

// ══════════════════════════════════════════════
// RAW KANIDM RESPONSE
// ══════════════════════════════════════════════

/** Raw Kanidm response — attrs são sempre string[] */
export interface KanidmEntry {
  attrs: Record<string, string[]>
}

// ══════════════════════════════════════════════
// PERSON
// ══════════════════════════════════════════════

export interface Person {
  id: string
  username: string
  displayName: string
  legalName?: string
  emails: string[]
  groups: string[]
  groupNames: string[]
  classes: string[]
  status: PersonStatus
  sshKeys: string[]
  accountExpiry?: Date
  accountValidFrom?: Date
  credentialStatus?: CredentialStatus
}

export type PersonStatus =
  | 'active'
  | 'expired'
  | 'not_yet_valid'
  | 'locked'
  | 'disabled'

export interface CredentialStatus {
  hasPassword: boolean
  hasTotp: boolean
  hasWebauthn: boolean
  hasPasskeys: boolean
  hasBackupCodes: boolean
  primaryMethod: 'password_mfa' | 'passkey' | 'password_only' | 'none'
}

export interface CreatePersonPayload {
  name: string
  displayname: string
  mail?: string[]
  legalname?: string
  groups?: string[]
}

export interface UpdatePersonPayload {
  displayname?: string
  legalname?: string
  mail?: string[]
  loginshell?: string
  ssh_publickey?: string[]
  account_expire?: string
  account_valid_from?: string
}

// ══════════════════════════════════════════════
// GROUP
// ══════════════════════════════════════════════

export interface Group {
  id: string
  name: string
  description?: string
  members: GroupMember[]
  memberOf: string[]
  memberCount: number
  isBuiltin: boolean
  isTenant: boolean
  classes: string[]
}

export interface GroupMember {
  id: string
  name: string
  type: 'person' | 'group' | 'service_account'
  displayName?: string
}

export interface CreateGroupPayload {
  name: string
  description?: string
  members?: string[]
}

// ══════════════════════════════════════════════
// SERVICE ACCOUNT
// ══════════════════════════════════════════════

export interface ServiceAccount {
  id: string
  name: string
  displayName: string
  description?: string
  groups: string[]
  groupNames: string[]
  apiTokens: ApiTokenInfo[]
  status: 'active' | 'expired' | 'disabled'
}

export interface ApiTokenInfo {
  tokenId: string
  label: string
  createdAt: Date
  expiresAt?: Date
}

export interface CreateServiceAccountPayload {
  name: string
  displayname: string
  description?: string
  groups?: string[]
}

// ══════════════════════════════════════════════
// OAUTH2 CLIENT
// ══════════════════════════════════════════════

export interface OAuth2Client {
  id: string
  name: string
  displayName: string
  type: 'basic' | 'public'
  landingUrl: string
  origins: string[]
  redirectUrls: string[]
  scopeMaps: ScopeMap[]
  supplementalScopeMaps: ScopeMap[]
  claimMaps: ClaimMap[]
  hasSecret: boolean
  isPkceEnabled: boolean
  classes: string[]
}

export interface ScopeMap {
  groupId: string
  groupName: string
  scopes: string[]
}

export interface ClaimMap {
  claimName: string
  groupId: string
  groupName: string
  values: string[]
}

export interface CreateOAuth2ClientPayload {
  name: string
  displayname: string
  origin_landing: string
  type: 'basic' | 'public'
}

// ══════════════════════════════════════════════
// SYSTEM / AUDIT
// ══════════════════════════════════════════════

export interface SystemStatus {
  kanidm: {
    status: 'ok' | 'error'
    version?: string
    domain?: string
    origin?: string
  }
  vault: {
    status: 'ok' | 'error' | 'unreachable'
    version?: string
  }
}

export interface AuditEvent {
  id: string
  timestamp: Date
  eventType: AuditEventType
  actor: string
  target?: string
  details: Record<string, unknown>
  sourceIp?: string
}

export type AuditEventType =
  | 'auth_success'
  | 'auth_failure'
  | 'person_created'
  | 'person_updated'
  | 'person_deleted'
  | 'group_created'
  | 'group_updated'
  | 'group_member_added'
  | 'group_member_removed'
  | 'oauth2_client_created'
  | 'oauth2_client_updated'
  | 'credential_reset'
  | 'account_locked'
  | 'account_unlocked'
  | 'token_generated'
  | 'token_revoked'
