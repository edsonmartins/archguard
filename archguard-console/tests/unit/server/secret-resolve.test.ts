import { describe, expect, it } from 'vitest'

/**
 * Pure helpers mirroring openbao-proxy kv path expansion / field pick.
 * Full HTTP tested in integration; keep unit free of network.
 */

function kvPathsToTry(ref: string): string[] {
  const paths = [ref]
  const m = ref.match(/^([^/]+)\/(?!data\/)(.+)$/)
  if (m && m[1] !== 'sys' && m[1] !== 'auth') {
    paths.push(`${m[1]}/data/${m[2]}`)
  }
  if (ref.includes('/data/')) {
    paths.push(ref.replace('/data/', '/'))
  }
  return [...new Set(paths)]
}

function pickSecretField(data: Record<string, unknown>): string | undefined {
  for (const k of ['password', 'value', 'secret', 'pass', 'token', 'key']) {
    const v = data[k]
    if (typeof v === 'string' && v.length > 0) return v
  }
  for (const v of Object.values(data)) {
    if (typeof v === 'string' && v.length > 0) return v
  }
  return undefined
}

describe('OpenBao secret_ref helpers', () => {
  it('expands secret/foo to secret/data/foo', () => {
    expect(kvPathsToTry('secret/archgate/targets/x')).toContain(
      'secret/data/archgate/targets/x',
    )
  })

  it('picks password field first', () => {
    expect(
      pickSecretField({ password: 's3cret', other: 'no' }),
    ).toBe('s3cret')
  })

  it('falls back to first string', () => {
    expect(pickSecretField({ note: 'only' })).toBe('only')
  })
})
