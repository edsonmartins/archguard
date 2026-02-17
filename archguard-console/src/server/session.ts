// src/server/session.ts

import { createCipheriv, createDecipheriv, randomBytes } from 'node:crypto'

const ALGORITHM = 'aes-256-gcm'

function getSessionSecret(): Buffer {
  const secret = process.env.SESSION_SECRET
  if (!secret || secret.length < 64) {
    throw new Error('SESSION_SECRET must be a 32-byte hex string (64 chars)')
  }
  return Buffer.from(secret, 'hex')
}

export function encryptSession(data: unknown): string {
  const key = getSessionSecret()
  const iv = randomBytes(12)
  const cipher = createCipheriv(ALGORITHM, key, iv)
  const json = JSON.stringify(data)
  const encrypted = Buffer.concat([
    cipher.update(json, 'utf8'),
    cipher.final(),
  ])
  const authTag = cipher.getAuthTag()
  return Buffer.concat([iv, authTag, encrypted]).toString('base64')
}

export function decryptSession<T = unknown>(cookie: string): T {
  const key = getSessionSecret()
  const buffer = Buffer.from(cookie, 'base64')
  const iv = buffer.subarray(0, 12)
  const authTag = buffer.subarray(12, 28)
  const encrypted = buffer.subarray(28)
  const decipher = createDecipheriv(ALGORITHM, key, iv)
  decipher.setAuthTag(authTag)
  const decrypted = Buffer.concat([
    decipher.update(encrypted),
    decipher.final(),
  ])
  return JSON.parse(decrypted.toString('utf8')) as T
}
