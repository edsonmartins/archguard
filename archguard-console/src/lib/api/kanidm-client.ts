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

export const oauth2Api = {
  list: async (): Promise<T.OAuth2Client[]> => {
    const raw = await api('GET', '/v1/oauth2')
    if (!raw || !Array.isArray(raw)) return []
    return (raw as T.KanidmEntry[]).map(normalizeOAuth2Client)
  },

  get: async (id: string): Promise<T.OAuth2Client> => {
    const raw = await api('GET', `/v1/oauth2/${encodeURIComponent(id)}`)
    return normalizeOAuth2Client(raw as T.KanidmEntry)
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

  delete: (id: string) =>
    api('DELETE', `/v1/oauth2/${encodeURIComponent(id)}`),

  getSecret: (id: string) =>
    api('GET', `/v1/oauth2/${encodeURIComponent(id)}/_basic_secret`),

  setScopeMap: (id: string, groupId: string, scopes: string[]) =>
    api('POST', `/v1/oauth2/${encodeURIComponent(id)}/_scopemap/${encodeURIComponent(groupId)}`, scopes),

  deleteScopeMap: (id: string, groupId: string) =>
    api('DELETE', `/v1/oauth2/${encodeURIComponent(id)}/_scopemap/${encodeURIComponent(groupId)}`),

  setSupScopeMap: (id: string, groupId: string, scopes: string[]) =>
    api('POST', `/v1/oauth2/${encodeURIComponent(id)}/_sup_scopemap/${encodeURIComponent(groupId)}`, scopes),

  setClaimMap: (
    id: string,
    claimName: string,
    groupId: string,
    values: string[],
  ) =>
    api(
      'POST',
      `/v1/oauth2/${encodeURIComponent(id)}/_claimmap/${encodeURIComponent(claimName)}/${encodeURIComponent(groupId)}`,
      values,
    ),

  addRedirectUrl: (id: string, url: string) =>
    api('POST', `/v1/oauth2/${encodeURIComponent(id)}/_attr/oauth2_rs_origin`, [url]),

  enableLocalhostRedirects: (id: string) =>
    api('PUT', `/v1/oauth2/${encodeURIComponent(id)}/_attr/oauth2_allow_localhost_redirect`, [
      'true',
    ]),

  preferShortUsername: (id: string) =>
    api('PUT', `/v1/oauth2/${encodeURIComponent(id)}/_attr/oauth2_prefer_short_username`, [
      'true',
    ]),
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
    return normalizeServiceAccount(raw as T.KanidmEntry)
  },

  create: (payload: T.CreateServiceAccountPayload) =>
    api('POST', '/v1/service_account', {
      attrs: {
        name: [payload.name],
        displayname: [payload.displayname],
        ...(payload.description && {
          description: [payload.description],
        }),
      },
    }),

  delete: (id: string) =>
    api('DELETE', `/v1/service_account/${encodeURIComponent(id)}`),

  generateToken: (id: string, label: string, expiry?: string) =>
    api('POST', `/v1/service_account/${encodeURIComponent(id)}/_api_token`, {
      label,
      expiry,
    }),

  revokeToken: (id: string, tokenId: string) =>
    api('DELETE', `/v1/service_account/${encodeURIComponent(id)}/_api_token/${encodeURIComponent(tokenId)}`),
}

// ── SYSTEM ──────────────────────────────────────

export const systemApi = {
  status: () => api('GET', '/status'),
  domain: () => api('GET', '/v1/domain'),
}
