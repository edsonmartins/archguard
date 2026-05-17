// src/server/logger.ts
//
// Structured logger with redaction. Tokens, cookies, secrets and PII-heavy
// fields are scrubbed before output so that operational logs never leak them.

import { pino } from 'pino'

const IS_PROD = process.env.NODE_ENV === 'production'

export const logger = pino({
  level: process.env.LOG_LEVEL || (IS_PROD ? 'info' : 'debug'),
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
  transport: IS_PROD
    ? undefined
    : {
        target: 'pino-pretty',
        options: { translateTime: 'HH:MM:ss', ignore: 'pid,hostname' },
      },
})
