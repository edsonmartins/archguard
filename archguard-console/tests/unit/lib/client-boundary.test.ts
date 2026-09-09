import { describe, expect, it } from 'vitest'
import { assertNoServerPersistence } from '../../../build/client-boundary'

describe('client persistence boundary', () => {
  it.each([
    '/app/node_modules/better-sqlite3/lib/database.js',
    '/app/src/server/db.ts',
    '/app/src/server/principal-revocation.ts?transformed',
    '/app/src/server/audit-outbox.ts',
    'C:\\app\\src\\server\\db.ts',
  ])('fails a build graph containing %s', (id) => {
    expect(() => assertNoServerPersistence([id])).toThrow('Server persistence reached client')
  })
  it('allows client code and transformed RPC entrypoints', () => {
    expect(() => assertNoServerPersistence([
      '/app/src/components/identity/person-list-page.tsx',
      '/app/src/server/person-read-fn.ts',
      '/app/src/server/auth.ts',
    ])).not.toThrow()
  })
})
