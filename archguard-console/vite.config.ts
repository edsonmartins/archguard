import { defineConfig } from 'vite'
import type { Plugin } from 'vite'
import { devtools } from '@tanstack/devtools-vite'
import { tanstackStart } from '@tanstack/react-start/plugin/vite'
import viteReact from '@vitejs/plugin-react'
import viteTsConfigPaths from 'vite-tsconfig-paths'
import { fileURLToPath, URL } from 'node:url'
import tailwindcss from '@tailwindcss/vite'

// SSR-only TanStack modules leak into the client bundle through barrel
// re-exports (router-core 1.160 imports `node:stream` and async_hooks at
// the top). For the client environment we replace their content with
// stubs that throw at runtime — they are only ever called server-side.
const SSR_ONLY_PATTERNS = [
  '@tanstack/router-core/dist/esm/ssr/transformStreamWithRouter',
  '@tanstack/react-router/dist/esm/ssr/renderRouterToStream',
  '@tanstack/start-storage-context/dist/esm/async-local-storage',
]

const SSR_ONLY_STUB =
  'const _ssrOnly = (..._a) => { throw new Error("SSR-only on client") };' +
  // transformStreamWithRouter
  'export const transformReadableStreamWithRouter = _ssrOnly;' +
  'export const transformPipeableStreamWithRouter = _ssrOnly;' +
  'export const transformStreamWithRouter = _ssrOnly;' +
  // renderRouterToStream
  'export const renderRouterToStream = _ssrOnly;' +
  // start-storage-context async-local-storage
  'export const getGlobalStartContext = () => undefined;' +
  'export const setGlobalStartContext = _ssrOnly;' +
  'export const runWithStartContext = _ssrOnly;' +
  'export const getStartContext = () => undefined;' +
  'export const setStartContext = _ssrOnly;' +
  'export default _ssrOnly;'

// Node built-ins pulled by server-only code in src/server/*. Stub them on the
// client so the client bundle parses; the runtime branches that touch them
// never run there because the plugin replaces server functions with RPC.
const NODE_BUILTIN_STUB =
  'const _serverOnly = () => { throw new Error("node:* used on client") };' +
  // node:crypto
  'export const createCipheriv = _serverOnly;' +
  'export const createDecipheriv = _serverOnly;' +
  'export const randomBytes = _serverOnly;' +
  'export const createHash = _serverOnly;' +
  // node:stream
  'export const Readable = class {};' +
  'export const Writable = class {};' +
  'export const ReadableStream = class {};' +
  // node:async_hooks
  'export const AsyncLocalStorage = class { run(_, fn) { return fn() } getStore() {} };' +
  // node:path
  'export const resolve = (...p) => p.join("/");' +
  'export const join = (...p) => p.join("/");' +
  'export const dirname = (p) => p;' +
  'export const basename = (p) => p;' +
  'export const extname = () => "";' +
  'export const normalize = (p) => p;' +
  // node:fs / node:fs/promises
  'export const mkdirSync = _serverOnly;' +
  'export const readFileSync = _serverOnly;' +
  'export const writeFileSync = _serverOnly;' +
  'export const readFile = _serverOnly;' +
  'export const writeFile = _serverOnly;' +
  'export const stat = _serverOnly;' +
  // node:url
  'export const fileURLToPath = (u) => String(u);' +
  'export const URL = globalThis.URL;' +
  'export default {};'

const NODE_BUILTINS_TO_STUB = new Set([
  'node:crypto',
  'node:stream',
  'node:stream/web',
  'node:async_hooks',
  'node:fs',
  'node:fs/promises',
  'node:path',
  'node:url',
  'node:http',
])

function stubServerOnlyForClient(): Plugin {
  return {
    name: 'archguard:stub-server-only-for-client',
    enforce: 'pre',
    applyToEnvironment(env) {
      return env.name === 'client'
    },
    resolveId(source) {
      if (NODE_BUILTINS_TO_STUB.has(source)) {
        return { id: `\0archguard:node-stub:${source}`, moduleSideEffects: false }
      }
      return null
    },
    load(id) {
      if (id.startsWith('\0archguard:node-stub:')) {
        return NODE_BUILTIN_STUB
      }
      if (SSR_ONLY_PATTERNS.some((p) => id.includes(p))) {
        return SSR_ONLY_STUB
      }
      return null
    },
  }
}

export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  plugins: [
    devtools(),
    viteTsConfigPaths({ projects: ['./tsconfig.json'] }),
    tailwindcss(),
    tanstackStart(),
    viteReact(),
    stubServerOnlyForClient(),
  ],
})
