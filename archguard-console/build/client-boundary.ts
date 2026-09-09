import type { Plugin } from 'vite'

export function assertNoServerPersistence(ids: Iterable<string>): void {
  const forbidden = [...ids].filter((id) => {
    const path = id.replace(/\\/g, '/')
    return /\/node_modules\/better-sqlite3\//.test(path) ||
      /\/src\/server\/(db|principal-revocation|audit-outbox)\.ts(?:\?|$)/.test(path)
  })
  if (forbidden.length) {
    throw new Error(`Server persistence reached client module graph:\n${forbidden.join('\n')}`)
  }
}

/** Check the graph before relying on tree-shaking or Node compatibility stubs. */
export function clientPersistenceBoundary(): Plugin {
  return {
    name: 'archguard:client-persistence-boundary',
    applyToEnvironment: (environment) => environment.name === 'client',
    generateBundle() {
      assertNoServerPersistence(this.getModuleIds())
    },
  }
}
