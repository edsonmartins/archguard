// src/server/logger.ts
//
// Structured logger with redaction. Tokens, cookies, secrets and PII-heavy
// fields are scrubbed before output so that operational logs never leak them.

import { pino, type Logger, type LoggerOptions } from 'pino'

const IS_PROD = process.env.NODE_ENV === 'production'
const IS_TEST = process.env.NODE_ENV === 'test' || process.env.VITEST === 'true'

function prettyTransport():
  | { target: string; options: Record<string, string> }
  | undefined {
  if (IS_PROD || IS_TEST) return undefined
  return {
    target: 'pino-pretty',
    options: { translateTime: 'HH:MM:ss', ignore: 'pid,hostname' },
  }
}

const options: LoggerOptions = {
  level: process.env.LOG_LEVEL || (IS_PROD ? 'info' : IS_TEST ? 'silent' : 'debug'),
  redact: {
    paths: [
      'token',
      'tokens',
      'access_token',
      'id_token',
      'refresh_token',
      'idToken',
      'accessToken',
      'refreshToken',
      'authorization',
      'Authorization',
      'cookie',
      'Cookie',
      'set-cookie',
      'sessionCookie',
      'password',
      'secret',
      'SESSION_SECRET',
      'ARCHGUARD_SA_TOKEN',
      '*.token',
      '*.access_token',
      '*.id_token',
      '*.refresh_token',
      '*.password',
      '*.secret',
    ],
    remove: true,
  },
}

/**
 * The comment here used to claim this degraded gracefully without pino-pretty,
 * but the transport was passed unconditionally and pino threw at construction —
 * which took the dev server down on boot and kept the E2E suite from ever
 * starting (audit 2026-07-28). Fall back to plain JSON instead of dying.
 *
 * The check is a try/catch rather than a module resolution because
 * `node:module` is not available in the client bundle this file is traced into.
 */
function build(): Logger {
  const transport = prettyTransport()
  if (!transport) return pino(options)
  try {
    return pino({ ...options, transport })
  } catch {
    const fallback = pino(options)
    fallback.warn('pino-pretty unavailable — falling back to JSON logs')
    return fallback
  }
}

export const logger = build()
