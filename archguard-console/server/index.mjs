// server/index.mjs
//
// Production entrypoint. Wraps the fetch handler emitted by `vite build`
// (dist/server/server.js) into a Node HTTP server, with healthcheck and
// static asset serving for the client bundle.

import { createServer } from 'node:http'
import { readFile, stat } from 'node:fs/promises'
import { extname, join, normalize, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { Readable } from 'node:stream'

import handler from '../dist/server/server.js'

const __dirname = fileURLToPath(new URL('.', import.meta.url))
const ROOT = resolve(__dirname, '..')
const CLIENT_DIR = join(ROOT, 'dist', 'client')

const PORT = Number(process.env.PORT ?? 3000)
const HOST = process.env.HOST ?? '0.0.0.0'

const MIME = {
  '.js': 'application/javascript; charset=utf-8',
  '.mjs': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.html': 'text/html; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.ico': 'image/x-icon',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
  '.txt': 'text/plain; charset=utf-8',
  '.webmanifest': 'application/manifest+json',
}

async function tryServeStatic(req, res) {
  if (req.method !== 'GET' && req.method !== 'HEAD') return false
  const url = new URL(req.url, `http://${req.headers.host}`)
  const safe = normalize(url.pathname).replace(/^(\.\.[/\\])+/, '')
  const filePath = join(CLIENT_DIR, safe)

  try {
    const s = await stat(filePath)
    if (!s.isFile()) return false
  } catch {
    return false
  }

  const body = await readFile(filePath)
  const type = MIME[extname(filePath).toLowerCase()] ?? 'application/octet-stream'

  // Hashed assets are immutable; everything else gets a short TTL.
  const cache = filePath.includes('/assets/')
    ? 'public, max-age=31536000, immutable'
    : 'public, max-age=300, must-revalidate'

  res.writeHead(200, { 'Content-Type': type, 'Cache-Control': cache })
  res.end(body)
  return true
}

function buildRequest(req) {
  const url = new URL(req.url, `http://${req.headers.host || 'localhost'}`)
  const headers = new Headers()
  for (const [k, v] of Object.entries(req.headers)) {
    if (v == null) continue
    if (Array.isArray(v)) for (const item of v) headers.append(k, item)
    else headers.set(k, v)
  }
  const init = { method: req.method, headers }
  if (req.method !== 'GET' && req.method !== 'HEAD') {
    init.body = Readable.toWeb(req)
    init.duplex = 'half'
  }
  return new Request(url, init)
}

async function writeResponse(webRes, res) {
  res.statusCode = webRes.status
  webRes.headers.forEach((v, k) => res.setHeader(k, v))
  if (!webRes.body) {
    res.end()
    return
  }
  Readable.fromWeb(webRes.body).pipe(res)
}

async function checkKanidm() {
  const url = process.env.ARCHGUARD_ID_URL
  if (!url) return { ok: false, reason: 'ARCHGUARD_ID_URL not set' }
  try {
    const res = await fetch(`${url}/status`, {
      signal: AbortSignal.timeout(3000),
    })
    return { ok: res.ok, status: res.status }
  } catch (err) {
    return { ok: false, reason: err?.message || 'unreachable' }
  }
}

const server = createServer(async (req, res) => {
  try {
    if (req.url === '/api/health' || req.url === '/healthz') {
      const checks = {
        sessionSecret: Boolean(process.env.SESSION_SECRET),
        saToken: Boolean(process.env.ARCHGUARD_SA_TOKEN),
        kanidm: await checkKanidm(),
      }
      const ok = checks.sessionSecret && checks.saToken && checks.kanidm.ok
      res.writeHead(ok ? 200 : 503, {
        'Content-Type': 'application/json',
      })
      res.end(JSON.stringify({ ok, checks }))
      return
    }

    if (await tryServeStatic(req, res)) return

    const request = buildRequest(req)
    const response = await handler.fetch(request)
    await writeResponse(response, res)
  } catch (err) {
    console.error('[server] unhandled error:', err)
    if (!res.headersSent) {
      res.writeHead(500, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({ error: 'internal_server_error' }))
    } else {
      res.destroy(err)
    }
  }
})

server.listen(PORT, HOST, () => {
  console.log(`[server] archguard-console listening on http://${HOST}:${PORT}`)
})
