// src/server/jwt.ts
//
// id_token verification against Kanidm's JWKS.
//
// Uses jose's RemoteJWKSet, which fetches and caches JWKS from the issuer
// (rotating keys are picked up on kid miss). OIDC discovery is performed once
// per process (issuer and jwks_uri are stable for a given Kanidm deployment).

import { createRemoteJWKSet, jwtVerify, type JWTPayload } from 'jose'

const KANIDM_URL = process.env.ARCHGUARD_ID_URL || 'https://localhost:8443'
const OIDC_CLIENT_ID = 'archguard-console'

interface DiscoveryDoc {
  issuer: string
  jwks_uri: string
}

let discoveryPromise: Promise<DiscoveryDoc> | null = null
let jwksPromise: ReturnType<typeof createRemoteJWKSet> | null = null

async function getDiscovery(): Promise<DiscoveryDoc> {
  if (!discoveryPromise) {
    const url = `${KANIDM_URL}/oauth2/openid/${OIDC_CLIENT_ID}/.well-known/openid-configuration`
    discoveryPromise = (async () => {
      const res = await fetch(url)
      if (!res.ok) {
        discoveryPromise = null
        throw new Error(
          `OIDC discovery failed (${res.status}): ${await res.text()}`,
        )
      }
      const doc = (await res.json()) as DiscoveryDoc
      if (!doc.issuer || !doc.jwks_uri) {
        discoveryPromise = null
        throw new Error('OIDC discovery missing issuer or jwks_uri')
      }
      return doc
    })()
  }
  return discoveryPromise
}

function getJwks(jwksUri: string) {
  if (!jwksPromise) {
    jwksPromise = createRemoteJWKSet(new URL(jwksUri), {
      cacheMaxAge: 60 * 60 * 1000,
      cooldownDuration: 30 * 1000,
    })
  }
  return jwksPromise
}

export async function verifyIdToken(token: string): Promise<JWTPayload> {
  const { issuer, jwks_uri } = await getDiscovery()
  const jwks = getJwks(jwks_uri)
  const { payload } = await jwtVerify(token, jwks, {
    issuer,
    audience: OIDC_CLIENT_ID,
  })
  return payload
}
