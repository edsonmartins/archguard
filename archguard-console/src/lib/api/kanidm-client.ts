// src/lib/api/kanidm-client.ts

import { kanidmApiFn } from '@/server/kanidm-proxy'
import {
  normalizePerson,
  normalizeCredentialStatus,
  normalizeGroup,
  normalizeOAuth2Client,
  normalizeServiceAccount,
} from './normalizers'
import type * as T from './types/kanidm'

async function api(
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE',
  path: string,
  body?: unknown,
) {
  return kanidmApiFn({ data: { method, path, body } })
}

// ── PERSONS ─────────────────────────────────────

export const personApi = {
  list: async (): Promise<T.Person[]> => {
    const raw = await api('GET', '/v1/person')
    if (!raw || !Array.isArray(raw)) return []
    return (raw as T.KanidmEntry[]).map(normalizePerson)
  },

  get: async (id: string): Promise<T.Person> => {
    const raw = await api('GET', `/v1/person/${encodeURIComponent(id)}`)
    return normalizePerson(raw as T.KanidmEntry)
  },

  create: async (payload: T.CreatePersonPayload) => {
    const result = await api('POST', '/v1/person', {
      attrs: {
        name: [payload.name],
        displayname: [payload.displayname],
        ...(payload.legalname && { legalname: [payload.legalname] }),
        ...(payload.mail?.length && { mail: payload.mail }),
      },
    })

    // After creation, add to groups if specified
    if (payload.groups?.length) {
      for (const groupId of payload.groups) {
        try {
          await api(
            'POST',
            `/v1/group/${encodeURIComponent(groupId)}/_attr/member`,
            [payload.name],
          )
        } catch {
          // Non-fatal: person created but group assignment failed
        }
      }
    }

    return result
  },

  delete: (id: string) =>
    api('DELETE', `/v1/person/${encodeURIComponent(id)}`),

  setAttr: (id: string, attr: string, values: string[]) =>
    api('PUT', `/v1/person/${encodeURIComponent(id)}/_attr/${encodeURIComponent(attr)}`, values),

  appendAttr: (id: string, attr: string, values: string[]) =>
    api('POST', `/v1/person/${encodeURIComponent(id)}/_attr/${encodeURIComponent(attr)}`, values),

  deleteAttr: (id: string, attr: string) =>
    api('DELETE', `/v1/person/${encodeURIComponent(id)}/_attr/${encodeURIComponent(attr)}`),

  credentialStatus: async (id: string): Promise<T.CredentialStatus> => {
    const raw = await api('GET', `/v1/person/${encodeURIComponent(id)}/_credential/_status`)
    return normalizeCredentialStatus(raw)
  },

  createResetToken: (id: string, ttl = 3600) =>
    api('POST', `/v1/person/${encodeURIComponent(id)}/_credential/_update_intent/${ttl}`),
}

// ── GROUPS ───────────────────────────────────────

export const groupApi = {
  list: async (): Promise<T.Group[]> => {
    const raw = await api('GET', '/v1/group')
    if (!raw || !Array.isArray(raw)) return []
    return (raw as T.KanidmEntry[]).map(normalizeGroup)
  },

  get: async (id: string): Promise<T.Group> => {
    const raw = await api('GET', `/v1/group/${encodeURIComponent(id)}`)
    return normalizeGroup(raw as T.KanidmEntry)
  },

  create: async (payload: T.CreateGroupPayload) => {
    const result = await api('POST', '/v1/group', {
      attrs: {
        name: [payload.name],
        ...(payload.description && {
          description: [payload.description],
        }),
      },
    })

    // After creation, add initial members if specified
    if (payload.members?.length) {
      try {
        await api(
          'POST',
          `/v1/group/${encodeURIComponent(payload.name)}/_attr/member`,
          payload.members,
        )
      } catch {
        // Non-fatal: group created but member assignment failed
      }
    }

    return result
  },

  delete: (id: string) =>
    api('DELETE', `/v1/group/${encodeURIComponent(id)}`),

  getMembers: (id: string) =>
    api('GET', `/v1/group/${encodeURIComponent(id)}/_attr/member`),

  addMembers: (id: string, memberIds: string[]) =>
    api('POST', `/v1/group/${encodeURIComponent(id)}/_attr/member`, memberIds),

  removeMembers: (id: string, memberIds: string[]) =>
    api('DELETE', `/v1/group/${encodeURIComponent(id)}/_attr/member`, memberIds),
}

// ── OAUTH2 ──────────────────────────────────────

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

// Kanidm v1.9 OAuth2 endpoints expect the client slug (name), not UUID.
// Most of our UI carries the UUID through routing — resolve to slug here
// so callers stay uniform.
async function resolveOAuth2Slug(id: string): Promise<string> {
  if (!UUID_RE.test(id)) return id
  const all = (await api('GET', '/v1/oauth2')) as T.KanidmEntry[] | null
  const match = (all ?? []).find((e) => e.attrs.uuid?.[0] === id)
  return match?.attrs.name?.[0] ?? id
}

export const oauth2Api = {
  list: async (): Promise<T.OAuth2Client[]> => {
    const raw = await api('GET', '/v1/oauth2')
    if (!raw || !Array.isArray(raw)) return []
    return (raw as T.KanidmEntry[]).map(normalizeOAuth2Client)
  },

  get: async (id: string): Promise<T.OAuth2Client> => {
    // Kanidm v1.9 only resolves /v1/oauth2/:id when :id is the slug (name),
    // not the UUID. Detail page links carry the UUID, so transparently
    // fall back to a list lookup when the direct GET returns null.
    const raw = await api('GET', `/v1/oauth2/${encodeURIComponent(id)}`)
    if (raw) return normalizeOAuth2Client(raw as T.KanidmEntry)
    const all = (await api('GET', '/v1/oauth2')) as T.KanidmEntry[] | null
    const match = (all ?? []).find(
      (e) => e.attrs.uuid?.[0] === id || e.attrs.name?.[0] === id,
    )
    if (!match) throw new Error(`OAuth2 client not found: ${id}`)
    return normalizeOAuth2Client(match)
  },

  createBasic: (payload: T.CreateOAuth2ClientPayload) =>
    api('POST', '/v1/oauth2/_basic', {
      attrs: {
        name: [payload.name],
        displayname: [payload.displayname],
        oauth2_rs_origin_landing: [payload.origin_landing],
      },
    }),

  createPublic: (payload: T.CreateOAuth2ClientPayload) =>
    api('POST', '/v1/oauth2/_public', {
      attrs: {
        name: [payload.name],
        displayname: [payload.displayname],
        oauth2_rs_origin_landing: [payload.origin_landing],
      },
    }),

  delete: async (id: string) => {
    const slug = await resolveOAuth2Slug(id)
    return api('DELETE', `/v1/oauth2/${encodeURIComponent(slug)}`)
  },

  getSecret: async (id: string) => {
    const slug = await resolveOAuth2Slug(id)
    return api('GET', `/v1/oauth2/${encodeURIComponent(slug)}/_basic_secret`)
  },

  setScopeMap: async (id: string, groupId: string, scopes: string[]) => {
    const slug = await resolveOAuth2Slug(id)
    return api('POST', `/v1/oauth2/${encodeURIComponent(slug)}/_scopemap/${encodeURIComponent(groupId)}`, scopes)
  },

  deleteScopeMap: async (id: string, groupId: string) => {
    const slug = await resolveOAuth2Slug(id)
    return api('DELETE', `/v1/oauth2/${encodeURIComponent(slug)}/_scopemap/${encodeURIComponent(groupId)}`)
  },

  setSupScopeMap: async (id: string, groupId: string, scopes: string[]) => {
    const slug = await resolveOAuth2Slug(id)
    return api('POST', `/v1/oauth2/${encodeURIComponent(slug)}/_sup_scopemap/${encodeURIComponent(groupId)}`, scopes)
  },

  setClaimMap: async (
    id: string,
    claimName: string,
    groupId: string,
    values: string[],
  ) => {
    const slug = await resolveOAuth2Slug(id)
    return api(
      'POST',
      `/v1/oauth2/${encodeURIComponent(slug)}/_claimmap/${encodeURIComponent(claimName)}/${encodeURIComponent(groupId)}`,
      values,
    )
  },

  deleteSupScopeMap: async (id: string, groupId: string) => {
    const slug = await resolveOAuth2Slug(id)
    return api('DELETE', `/v1/oauth2/${encodeURIComponent(slug)}/_sup_scopemap/${encodeURIComponent(groupId)}`)
  },

  deleteClaimMap: async (id: string, claimName: string, groupId: string) => {
    const slug = await resolveOAuth2Slug(id)
    return api(
      'DELETE',
      `/v1/oauth2/${encodeURIComponent(slug)}/_claimmap/${encodeURIComponent(claimName)}/${encodeURIComponent(groupId)}`,
    )
  },

  addRedirectUrl: async (id: string, url: string) => {
    const slug = await resolveOAuth2Slug(id)
    return api('POST', `/v1/oauth2/${encodeURIComponent(slug)}/_attr/oauth2_rs_origin`, [url])
  },

  enableLocalhostRedirects: async (id: string) => {
    const slug = await resolveOAuth2Slug(id)
    return api('PUT', `/v1/oauth2/${encodeURIComponent(slug)}/_attr/oauth2_allow_localhost_redirect`, [
      'true',
    ])
  },

  preferShortUsername: async (id: string) => {
    const slug = await resolveOAuth2Slug(id)
    return api('PUT', `/v1/oauth2/${encodeURIComponent(slug)}/_attr/oauth2_prefer_short_username`, [
      'true',
    ])
  },
}

// ── SERVICE ACCOUNTS ────────────────────────────

export const serviceAccountApi = {
  list: async (): Promise<T.ServiceAccount[]> => {
    const raw = await api('GET', '/v1/service_account')
    if (!raw || !Array.isArray(raw)) return []
    return (raw as T.KanidmEntry[]).map(normalizeServiceAccount)
  },

  get: async (id: string): Promise<T.ServiceAccount> => {
    const raw = await api('GET', `/v1/service_account/${encodeURIComponent(id)}`)
    const sa = normalizeServiceAccount(raw as T.KanidmEntry)
    const tokens = await api(
      'GET',
      `/v1/service_account/${encodeURIComponent(id)}/_api_token`,
    )
    if (Array.isArray(tokens)) {
      sa.apiTokens = (tokens as Array<{
        token_id: string
        label: string
        expiry: string | null
        issued_at: number
      }>).map((t) => ({
        tokenId: t.token_id,
        label: t.label,
        createdAt: new Date(t.issued_at * 1000),
        expiresAt: t.expiry ? new Date(t.expiry) : undefined,
      }))
    }
    return sa
  },

  create: (payload: T.CreateServiceAccountPayload) =>
    // Kanidm v1.9 requires `entry_managed_by` for token issuance to work
    // (and silently drops `displayname` if it isn't set).
    api('POST', '/v1/service_account', {
      attrs: {
        name: [payload.name],
        displayname: [payload.displayname],
        entry_managed_by: ['idm_admins'],
        ...(payload.description && {
          description: [payload.description],
        }),
      },
    }),

  delete: (id: string) =>
    api('DELETE', `/v1/service_account/${encodeURIComponent(id)}`),

  generateToken: (id: string, label: string, expiry?: string) =>
    // Kanidm v1.9 requires every field on this body — `expiry` must be
    // present (null = no expiration) and `read_write` is mandatory.
    api('POST', `/v1/service_account/${encodeURIComponent(id)}/_api_token`, {
      label,
      expiry: expiry ?? null,
      read_write: true,
    }),

  revokeToken: (id: string, tokenId: string) =>
    api('DELETE', `/v1/service_account/${encodeURIComponent(id)}/_api_token/${encodeURIComponent(tokenId)}`),
}

// ── PERSON SSH KEYS ─────────────────────────────

export const sshKeyApi = {
  list: async (personId: string): Promise<Record<string, string>> => {
    const raw = await api('GET', `/v1/person/${encodeURIComponent(personId)}/_ssh_pubkeys`)
    return (raw as Record<string, string>) ?? {}
  },

  add: (personId: string, tag: string, publicKey: string) =>
    api('POST', `/v1/person/${encodeURIComponent(personId)}/_ssh_pubkeys`, {
      tag,
      key: publicKey,
    }),

  delete: (personId: string, tag: string) =>
    api('DELETE', `/v1/person/${encodeURIComponent(personId)}/_ssh_pubkeys/${encodeURIComponent(tag)}`),
}

// ── RECYCLE BIN ─────────────────────────────────

export const recycleBinApi = {
  list: async (): Promise<T.RecycleBinEntry[]> => {
    const raw = await api('GET', '/v1/recycle_bin')
    if (!raw || !Array.isArray(raw)) return []
    return (raw as T.KanidmEntry[]).map((entry) => ({
      id: entry.attrs.uuid?.[0] ?? '',
      name: entry.attrs.name?.[0] ?? entry.attrs.spn?.[0] ?? 'unknown',
      type: (entry.attrs.class ?? []).find((c) =>
        ['person', 'group', 'service_account', 'oauth2_resource_server'].includes(c),
      ) ?? 'unknown',
      classes: entry.attrs.class ?? [],
      attrs: entry.attrs,
    }))
  },

  revive: (id: string) =>
    api('POST', `/v1/recycle_bin/${encodeURIComponent(id)}/_revive`),
}

// ── ACCOUNT POLICY ──────────────────────────────

export const accountPolicyApi = {
  get: async (groupId: string): Promise<T.AccountPolicy> => {
    const raw = await api('GET', `/v1/group/${encodeURIComponent(groupId)}`)
    const entry = raw as T.KanidmEntry
    const a = entry.attrs
    return {
      credentialTypeMinimum: (a.credential_type_minimum?.[0] as T.AccountPolicy['credentialTypeMinimum']) ?? 'any',
      authSessionExpiry: a.authsession_expiry?.[0] ? Number(a.authsession_expiry[0]) : undefined,
      privilegeExpiry: a.privilege_expiry?.[0] ? Number(a.privilege_expiry[0]) : undefined,
    }
  },

  setCredentialMinimum: (groupId: string, value: T.AccountPolicy['credentialTypeMinimum']) =>
    api('PUT', `/v1/group/${encodeURIComponent(groupId)}/_attr/credential_type_minimum`, [value]),

  setAuthSessionExpiry: (groupId: string, seconds: number) =>
    api('PUT', `/v1/group/${encodeURIComponent(groupId)}/_attr/authsession_expiry`, [String(seconds)]),

  setPrivilegeExpiry: (groupId: string, seconds: number) =>
    api('PUT', `/v1/group/${encodeURIComponent(groupId)}/_attr/privilege_expiry`, [String(seconds)]),
}

// ── SYSTEM ──────────────────────────────────────

export const systemApi = {
  status: () => api('GET', '/status'),
  domain: () => api('GET', '/v1/domain'),

  domainAttr: async (attr: string): Promise<string[]> => {
    const raw = await api('GET', `/v1/domain/_attr/${encodeURIComponent(attr)}`)
    if (Array.isArray(raw)) return raw as string[]
    return []
  },

  setDomainAttr: (attr: string, values: string[]) =>
    api('PUT', `/v1/domain/_attr/${encodeURIComponent(attr)}`, values),
}
