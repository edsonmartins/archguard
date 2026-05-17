import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { encryptSession, decryptSession } from '@/server/session'

const VALID_SECRET =
  '0000000000000000000000000000000000000000000000000000000000000000'

describe('session encryption (AES-256-GCM)', () => {
  const original = process.env.SESSION_SECRET

  beforeEach(() => {
    process.env.SESSION_SECRET = VALID_SECRET
  })

  afterEach(() => {
    process.env.SESSION_SECRET = original
  })

  it('round-trips an object payload', () => {
    const payload = {
      user: { id: 'abc', name: 'alice' },
      groups: ['archguard_admins'],
      n: 42,
    }
    const decoded = decryptSession(encryptSession(payload))
    expect(decoded).toEqual(payload)
  })

  it('produces a different ciphertext per encryption (random IV)', () => {
    const payload = { foo: 'bar' }
    const a = encryptSession(payload)
    const b = encryptSession(payload)
    expect(a).not.toBe(b)
    expect(decryptSession(a)).toEqual(decryptSession(b))
  })

  it('rejects tampered ciphertext (auth tag check)', () => {
    const enc = encryptSession({ x: 1 })
    const buf = Buffer.from(enc, 'base64')
    // Flip a bit in the encrypted body (after iv:12 + tag:16)
    buf[30] = buf[30] ^ 0x01
    const tampered = buf.toString('base64')
    expect(() => decryptSession(tampered)).toThrow()
  })

  it('rejects ciphertext encrypted with a different key', () => {
    process.env.SESSION_SECRET =
      'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
    const enc = encryptSession({ x: 1 })

    process.env.SESSION_SECRET = VALID_SECRET
    expect(() => decryptSession(enc)).toThrow()
  })

  it('throws if SESSION_SECRET is missing', () => {
    delete process.env.SESSION_SECRET
    expect(() => encryptSession({ x: 1 })).toThrow(/SESSION_SECRET/)
  })

  it('throws if SESSION_SECRET is too short', () => {
    process.env.SESSION_SECRET = 'deadbeef'
    expect(() => encryptSession({ x: 1 })).toThrow(/SESSION_SECRET/)
  })
})
