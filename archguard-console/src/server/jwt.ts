// src/server/jwt.ts
//
// id_token verification against the IdP's JWKS.
//
// Uses jose's RemoteJWKSet, which fetches and caches JWKS from the issuer
// (rotating keys are picked up on kid miss). The issuer and jwks_uri come from
// OIDC discovery, so this file works against Kanidm (issuer per client) and
// ArchGuard (single issuer) without knowing which is active.

import { createRemoteJWKSet, jwtVerify, type JWTPayload } from 'jose'
import { discover } from './idp/discovery'

const OIDC_CLIENT_ID = 'archguard-console'

let jwksPromise: ReturnType<typeof createRemoteJWKSet> | null = null

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
  const { issuer, jwks_uri } = await discover(OIDC_CLIENT_ID)
  const jwks = getJwks(jwks_uri)
  const { payload } = await jwtVerify(token, jwks, {
    issuer,
    audience: OIDC_CLIENT_ID,
  })
  return payload
}
