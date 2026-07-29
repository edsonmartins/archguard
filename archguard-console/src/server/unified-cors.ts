// CORS for the unified BFF (/api/unified/v1/*).
//
// These endpoints answer with the operator's catalog and with short-lived
// session/tunnel material, so they are credentialed. The previous helper
// reflected whatever `Origin` the caller sent and paired it with
// `Access-Control-Allow-Credentials: true`, which is an allow-any-site policy
// (audit 2026-07-28). Cross-site cookie sends were already blocked by
// SameSite=lax, but the reflection removed the second layer for any caller
// holding a Bearer token.
//
// Policy now: an explicit allowlist, or no CORS headers at all. Same-origin
// callers (the console-hosted UI) never need them, and ArchGate Connect is not
// a browser, so CORS does not apply to it either.

/** Origins allowed to make credentialed calls, from env. Empty = same-origin only. */
export function allowedOrigins(): string[] {
  const raw =
    process.env.UNIFIED_UI_ORIGIN || process.env.CORS_ALLOW_ORIGIN || ''
  return raw
    .split(',')
    .map((o) => o.trim().replace(/\/$/, ''))
    .filter(Boolean)
}

export type UnifiedCorsOptions = {
  methods: string
  headers?: string
}

/**
 * Returns CORS headers only when the request Origin is on the allowlist.
 * An unknown or absent Origin yields no CORS headers, so the browser applies
 * its default same-origin rule.
 */
export function unifiedCorsHeaders(
  request: Request,
  opts: UnifiedCorsOptions,
): Record<string, string> {
  const origin = (request.headers.get('Origin') || '').replace(/\/$/, '')
  if (!origin) return {}

  const allow = allowedOrigins()
  if (!allow.includes(origin)) return {}

  return {
    'Access-Control-Allow-Origin': origin,
    'Access-Control-Allow-Credentials': 'true',
    'Access-Control-Allow-Headers': opts.headers ?? 'Content-Type, Authorization',
    'Access-Control-Allow-Methods': opts.methods,
    // Caches must not serve one origin's response to another.
    Vary: 'Origin',
  }
}
