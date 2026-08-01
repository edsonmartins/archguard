// OIDC discovery, per IdP.
//
// Kanidm publishes one issuer per client:
//   https://id.archgate.com.br/oauth2/openid/<client>/.well-known/openid-configuration
//
// ArchGuard (Casdoor) publishes a single issuer for the deployment:
//   https://app.archguard.com.br/.well-known/openid-configuration
//
// Everything downstream — issuer, jwks_uri, userinfo_endpoint, token_endpoint —
// comes from the document, so this URL is the only shape that has to change.
// In particular the userinfo path differs (`<issuer>/userinfo` on Kanidm,
// `/api/userinfo` on Casdoor) and must never be hand-built again.

import { idpKind } from './index'

export type DiscoveryDoc = {
  issuer: string
  jwks_uri: string
  userinfo_endpoint?: string
  token_endpoint?: string
  end_session_endpoint?: string
}

function base(explicit?: string): string {
  return (
    explicit ||
    process.env.ARCHGUARD_ID_URL ||
    'https://localhost:8443'
  ).replace(/\/$/, '')
}

/** Discovery URL for a given OIDC client on the active IdP. */
export function discoveryUrl(clientId: string, explicitBase?: string): string {
  const root = base(explicitBase)
  return idpKind() === 'archguard'
    ? `${root}/.well-known/openid-configuration`
    : `${root}/oauth2/openid/${clientId}/.well-known/openid-configuration`
}

const cache = new Map<string, Promise<DiscoveryDoc>>()

/** Fetches and caches the discovery document for a client. */
export async function discover(
  clientId: string,
  explicitBase?: string,
): Promise<DiscoveryDoc> {
  const url = discoveryUrl(clientId, explicitBase)
  const hit = cache.get(url)
  if (hit) return hit

  const pending = (async () => {
    const res = await fetch(url)
    if (!res.ok) {
      throw new Error(`OIDC discovery failed (${res.status}): ${await res.text()}`)
    }
    const doc = (await res.json()) as DiscoveryDoc
    if (!doc.issuer || !doc.jwks_uri) {
      throw new Error('OIDC discovery missing issuer or jwks_uri')
    }
    return doc
  })()

  // Only cache success: a transient failure must not pin a broken promise.
  pending.catch(() => cache.delete(url))
  cache.set(url, pending)
  return pending
}

/** Test seam — drops the memoized documents. */
export function resetDiscoveryCache(): void {
  cache.clear()
}
