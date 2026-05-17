// src/lib/api/normalizers.ts

import type {
  KanidmEntry,
  Person,
  PersonStatus,
  CredentialStatus,
  Group,
  GroupMember,
  OAuth2Client,
  ServiceAccount,
  ScopeMap,
  ClaimMap,
} from './types/kanidm'
import { BUILTIN_GROUPS } from '../utils/constants'

export function normalizePerson(raw: KanidmEntry): Person {
  const a = raw.attrs
  // memberof contains UUIDs; the Kanidm list API also provides
  // memberof attribute which often contains group names or UUIDs.
  // We derive groupNames from the memberof attribute values.
  const memberOf = a.memberof ?? []

  return {
    id: a.uuid?.[0] ?? '',
    username: a.name?.[0] ?? '',
    displayName: a.displayname?.[0] ?? '',
    legalName: a.legalname?.[0],
    emails: a.mail ?? [],
    groups: memberOf,
    groupNames: memberOf,
    classes: a.class ?? [],
    status: derivePersonStatus(a),
    sshKeys: a.ssh_publickey ?? [],
    accountExpiry: a.account_expire?.[0]
      ? new Date(a.account_expire[0])
      : undefined,
    accountValidFrom: a.account_valid_from?.[0]
      ? new Date(a.account_valid_from[0])
      : undefined,
  }
}

export function normalizeCredentialStatus(raw: unknown): CredentialStatus {
  // Kanidm returns credential status in various formats;
  // normalize to our CredentialStatus interface
  if (!raw || typeof raw !== 'object') {
    return {
      hasPassword: false,
      hasTotp: false,
      hasWebauthn: false,
      hasPasskeys: false,
      hasBackupCodes: false,
      primaryMethod: 'none',
    }
  }

  const data = raw as Record<string, unknown>

  // Handle array-of-credentials format from Kanidm
  const creds = Array.isArray(data.creds) ? data.creds : []

  const hasPassword = creds.some(
    (c: Record<string, unknown>) =>
      c.type === 'password' || c.type === 'generatedpassword',
  )
  const hasTotp = creds.some(
    (c: Record<string, unknown>) => c.type === 'totp',
  )
  const hasWebauthn = creds.some(
    (c: Record<string, unknown>) =>
      c.type === 'webauthn' || c.type === 'securitykey',
  )
  const hasPasskeys = creds.some(
    (c: Record<string, unknown>) =>
      c.type === 'passkey' || c.type === 'attested_passkey',
  )
  const hasBackupCodes = creds.some(
    (c: Record<string, unknown>) => c.type === 'backup_code',
  )

  let primaryMethod: CredentialStatus['primaryMethod'] = 'none'
  if (hasPasskeys) {
    primaryMethod = 'passkey'
  } else if (hasPassword && (hasTotp || hasWebauthn)) {
    primaryMethod = 'password_mfa'
  } else if (hasPassword) {
    primaryMethod = 'password_only'
  }

  return {
    hasPassword,
    hasTotp,
    hasWebauthn,
    hasPasskeys,
    hasBackupCodes,
    primaryMethod,
  }
}

/**
 * Extract tenant prefix from a group name using the naming convention:
 *   {tenant}_admins, {tenant}_users, {tenant}_* → tenant
 *   idm_*, system_*, archguard_* → null (builtin)
 *   standalone name without _ → could be tenant root group
 */
export function extractTenantPrefix(groupName: string): string | null {
  // Builtin groups are never tenants
  if (BUILTIN_GROUPS.has(groupName)) return null
  // idm_ and system_ prefixed groups are Kanidm internal
  if (groupName.startsWith('idm_') || groupName.startsWith('system_')) return null
  // archguard_ prefixed groups are platform-level
  if (groupName.startsWith('archguard_')) return null

  // Groups with suffixes: {tenant}_{role} → extract tenant
  const suffixes = ['_admins', '_users', '_service_desk', '_viewers', '_developers', '_operators']
  for (const suffix of suffixes) {
    if (groupName.endsWith(suffix)) {
      return groupName.slice(0, -suffix.length)
    }
  }

  // Group without known suffix but containing underscore: try to extract prefix
  // e.g., "acme_custom_role" → "acme" (first segment before _)
  if (groupName.includes('_')) {
    // Check if first segment is a known tenant by looking at naming pattern
    // For safety, return null — only well-known suffixes identify tenants
    return null
  }

  // Standalone name (no underscore): this is a tenant root group
  // e.g., "acme" is the root group for tenant "acme"
  return groupName
}

export function normalizeGroup(raw: KanidmEntry): Group {
  const a = raw.attrs
  const name = a.name?.[0] ?? ''

  // Parse member attribute to create GroupMember entries
  const memberUuids = a.member ?? []
  const members: GroupMember[] = memberUuids.map((uuid) => ({
    id: uuid,
    name: uuid,
    type: 'person' as const,
  }))

  return {
    id: a.uuid?.[0] ?? '',
    name,
    description: a.description?.[0],
    members,
    memberOf: a.memberof ?? [],
    memberCount: memberUuids.length,
    isBuiltin:
      BUILTIN_GROUPS.has(name) ||
      (a.class ?? []).includes('system_protected'),
    tenantName: extractTenantPrefix(name),
    classes: a.class ?? [],
  }
}

export function normalizeOAuth2Client(raw: KanidmEntry): OAuth2Client {
  const a = raw.attrs
  const classes = a.class ?? []
  return {
    id: a.uuid?.[0] ?? '',
    name: a.name?.[0] ?? '',
    displayName: a.displayname?.[0] ?? '',
    type: classes.includes('oauth2_resource_server_basic') ? 'basic' : 'public',
    landingUrl: a.oauth2_rs_origin_landing?.[0] ?? '',
    origins: a.oauth2_rs_origin ?? [],
    redirectUrls: a.oauth2_rs_origin_landing
      ? [a.oauth2_rs_origin_landing[0] ?? '', ...(a.oauth2_rs_origin ?? [])]
      : a.oauth2_rs_origin ?? [],
    scopeMaps: parseScopeMaps(a.oauth2_rs_scope_map ?? []),
    supplementalScopeMaps: parseScopeMaps(a.oauth2_rs_sup_scope_map ?? []),
    claimMaps: parseClaimMaps(a.oauth2_rs_claim_map ?? []),
    hasSecret: classes.includes('oauth2_resource_server_basic'),
    isPkceEnabled: true,
    classes,
  }
}

export function normalizeServiceAccount(raw: KanidmEntry): ServiceAccount {
  const a = raw.attrs
  const memberOf = a.memberof ?? []
  return {
    id: a.uuid?.[0] ?? '',
    name: a.name?.[0] ?? '',
    displayName: a.displayname?.[0] ?? a.name?.[0] ?? '',
    description: a.description?.[0],
    groups: memberOf,
    groupNames: memberOf,
    apiTokens: [],
    status: 'active',
  }
}

function derivePersonStatus(
  attrs: Record<string, string[]>,
): PersonStatus {
  const now = new Date()
  if (
    attrs.account_expire?.[0] &&
    new Date(attrs.account_expire[0]) < now
  ) {
    return 'expired'
  }
  if (
    attrs.account_valid_from?.[0] &&
    new Date(attrs.account_valid_from[0]) > now
  ) {
    return 'not_yet_valid'
  }
  return 'active'
}

export function parseScopeMaps(raw: string[]): ScopeMap[] {
  return raw.map((entry) => {
    // Format: "groupUuid:scope1,scope2"
    const [groupId = '', scopeStr = ''] = entry.split(':')
    return {
      groupId,
      groupName: '',
      scopes: scopeStr.split(',').filter(Boolean),
    }
  })
}

export function parseClaimMaps(raw: string[]): ClaimMap[] {
  return raw.map((entry) => {
    // Format: "claimName:groupUuid:value1,value2"
    const parts = entry.split(':')
    return {
      claimName: parts[0] ?? '',
      groupId: parts[1] ?? '',
      groupName: '',
      values: (parts[2] ?? '').split(',').filter(Boolean),
    }
  })
}
