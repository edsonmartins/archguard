// Mentors Axis client — aligned with mentors-axis-server-api (port 9030, Archbase JWT)
// Repo: /Users/edsonmartins/desenvolvimento/mentors-axis-server-api
//
// Auth (Archbase):
//   POST /api/v1/auth/authenticate  { email, password } → { access_token }
// Proprietário:
//   GET  /api/v1/proprietario/findAll?page=0&size=100  (+ Authorization Bearer, X-TENANT-IDs)

export type AxisProprietario = {
  id: string
  code?: string
  descricao?: string
  cnpj?: string
  cpf?: string
  status?: string | { name?: string }
  adminEmail?: string
  email?: string
  onPremiseId?: string
  defaultGroupId?: string
  groupMapping?: string
}

const AXIS_URL = (process.env.MENTORS_AXIS_URL || '').replace(/\/$/, '')
const AXIS_TOKEN = process.env.MENTORS_AXIS_TOKEN || ''
const AXIS_EMAIL = process.env.MENTORS_AXIS_EMAIL || process.env.MENTORS_AXIS_USER || ''
const AXIS_PASSWORD = process.env.MENTORS_AXIS_PASSWORD || ''
const AXIS_TENANT_ID =
  process.env.MENTORS_AXIS_TENANT_ID ||
  'a9f814d2-4dae-41f3-851b-8aa3d4706561' // default tenant in application.yml
const AXIS_MOCK =
  process.env.MENTORS_AXIS_MOCK === '1' ||
  process.env.MENTORS_AXIS_MOCK === 'true'

let cachedToken: { token: string; exp: number } | null = null

export function mentorsAxisConfigured(): boolean {
  return Boolean(AXIS_URL) || AXIS_MOCK
}

export function mentorsAxisMode(): 'live' | 'mock' | 'disabled' {
  if (AXIS_URL) return 'live'
  if (AXIS_MOCK) return 'mock'
  return 'disabled'
}

const MOCK_PROPRIETARIOS: AxisProprietario[] = [
  {
    id: 'mock-rio-quality',
    code: 'rio_quality',
    descricao: 'Rio Quality',
    cnpj: '00.000.000/0001-00',
    status: 'ATIVO',
  },
  {
    id: 'mock-grupo-marra',
    code: 'grupo_marra',
    descricao: 'Grupo Marra',
    cnpj: '00.000.000/0002-00',
    status: 'ATIVO',
  },
  {
    id: 'mock-acme',
    code: 'acme',
    descricao: 'ACME Indústria (exemplo Axis)',
    status: 'ATIVO',
  },
]

function baseHeaders(token?: string): Record<string, string> {
  const h: Record<string, string> = {
    Accept: 'application/json',
    'Content-Type': 'application/json',
  }
  // Archbase multi-tenant (application.yml cors allowed-headers)
  if (AXIS_TENANT_ID) {
    h['X-TENANT-IDs'] = AXIS_TENANT_ID
  }
  if (token) {
    h.Authorization = token.startsWith('Bearer ') ? token : `Bearer ${token}`
  }
  return h
}

/**
 * Login via Archbase AuthenticationController:
 * POST /api/v1/auth/authenticate { email, password }
 * Fallback: whitelist mentions /api/v1/autenticacao/**
 */
async function loginForToken(): Promise<string> {
  if (AXIS_TOKEN) {
    return AXIS_TOKEN.startsWith('Bearer ')
      ? AXIS_TOKEN.slice(7)
      : AXIS_TOKEN
  }
  if (!AXIS_EMAIL || !AXIS_PASSWORD) {
    throw new Error(
      'Mentors Axis live: defina MENTORS_AXIS_TOKEN ou MENTORS_AXIS_EMAIL + MENTORS_AXIS_PASSWORD',
    )
  }
  if (cachedToken && cachedToken.exp > Date.now() + 60_000) {
    return cachedToken.token
  }

  const paths = [
    '/api/v1/auth/authenticate', // Archbase 1.0.26
    '/api/v1/autenticacao/authenticate',
    '/api/v1/autenticacao/login',
  ]
  let lastErr = 'login failed'
  for (const path of paths) {
    try {
      const res = await fetch(`${AXIS_URL}${path}`, {
        method: 'POST',
        headers: baseHeaders(),
        body: JSON.stringify({
          email: AXIS_EMAIL,
          password: AXIS_PASSWORD,
        }),
      })
      const text = await res.text()
      if (!res.ok) {
        lastErr = `${path} → ${res.status} ${text.slice(0, 200)}`
        continue
      }
      const data = JSON.parse(text) as {
        access_token?: string
        accessToken?: string
        token?: string
      }
      const token =
        data.access_token || data.accessToken || data.token || ''
      if (!token) {
        lastErr = `${path}: resposta sem access_token`
        continue
      }
      cachedToken = {
        token,
        exp: Date.now() + 12 * 60 * 60 * 1000,
      }
      return token
    } catch (e) {
      lastErr = (e as Error).message
    }
  }
  throw new Error(`Axis login: ${lastErr}`)
}

async function axisGet(path: string): Promise<unknown> {
  if (!AXIS_URL) throw new Error('MENTORS_AXIS_URL não configurada')
  const token = await loginForToken()
  const res = await fetch(`${AXIS_URL}${path}`, {
    method: 'GET',
    headers: baseHeaders(token),
  })
  const text = await res.text()
  if (!res.ok) {
    throw new Error(`Axis GET ${path}: ${res.status} ${text.slice(0, 300)}`)
  }
  try {
    return JSON.parse(text)
  } catch {
    throw new Error(`Axis GET ${path}: JSON inválido`)
  }
}

function normalizeProprietario(raw: Record<string, unknown>): AxisProprietario {
  const status = raw.status
  let statusStr: string | undefined
  if (typeof status === 'string') statusStr = status
  else if (status && typeof status === 'object' && 'name' in (status as object)) {
    statusStr = String((status as { name?: string }).name)
  } else if (status != null) statusStr = String(status)

  return {
    id: String(raw.id ?? raw.id_proprietario ?? ''),
    code:
      raw.code != null
        ? String(raw.code)
        : raw.cd_proprietario != null
          ? String(raw.cd_proprietario)
          : undefined,
    descricao: raw.descricao != null ? String(raw.descricao) : undefined,
    cnpj: raw.cnpj != null ? String(raw.cnpj) : undefined,
    cpf: raw.cpf != null ? String(raw.cpf) : undefined,
    status: statusStr,
    adminEmail:
      raw.adminEmail != null
        ? String(raw.adminEmail)
        : raw.admin_email != null
          ? String(raw.admin_email)
          : raw.dsUsuarioEmail != null
            ? String(raw.dsUsuarioEmail)
            : undefined,
    onPremiseId:
      raw.onPremiseId != null ? String(raw.onPremiseId) : undefined,
    defaultGroupId:
      raw.defaultGroupId != null ? String(raw.defaultGroupId) : undefined,
    groupMapping:
      raw.groupMapping != null ? String(raw.groupMapping) : undefined,
  }
}

/**
 * Page of proprietarios from Axis.
 * Controller: GET /api/v1/proprietario/findAll?page=&size=
 * Returns Spring Page { content: ProprietarioDto[] }
 */
export async function listProprietarios(
  page = 0,
  size = 100,
): Promise<AxisProprietario[]> {
  if (mentorsAxisMode() === 'mock') {
    return MOCK_PROPRIETARIOS
  }
  if (mentorsAxisMode() === 'disabled') {
    throw new Error(
      'Mentors Axis desligado: MENTORS_AXIS_URL ou MENTORS_AXIS_MOCK=1',
    )
  }

  const all: AxisProprietario[] = []
  let p = page
  let totalPages = 1

  do {
    const path = `/api/v1/proprietario/findAll?page=${p}&size=${size}`
    const data = (await axisGet(path)) as
      | { content?: Record<string, unknown>[]; totalPages?: number }
      | Record<string, unknown>[]

    if (Array.isArray(data)) {
      return data.map((x) => normalizeProprietario(x))
    }
    const content = data.content || []
    all.push(...content.map((x) => normalizeProprietario(x)))
    totalPages = data.totalPages ?? 1
    p++
  } while (p < totalPages && p < 20) // safety cap

  return all
}

export function slugFromProprietario(p: AxisProprietario): string {
  const code = (p.code || '').toString().trim()
  const looksUuidFrag = /^[0-9a-f]{6,}$/i.test(code) || /^\d+$/.test(code)
  const raw = (
    (!looksUuidFrag && code) ||
    p.descricao ||
    code ||
    p.id ||
    'cliente'
  )
    .toString()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
  return raw || p.id.replace(/[^a-z0-9_]/gi, '_').toLowerCase()
}

export function axisConnectionInfo() {
  return {
    url: AXIS_URL || null,
    mode: mentorsAxisMode(),
    tenant_id: AXIS_TENANT_ID,
    auth:
      AXIS_TOKEN
        ? 'token'
        : AXIS_EMAIL
          ? 'password'
          : mentorsAxisMode() === 'mock'
            ? 'mock'
            : 'none',
    repo: 'mentors-axis-server-api',
    endpoints: {
      login: 'POST /api/v1/auth/authenticate',
      list: 'GET /api/v1/proprietario/findAll?page&size',
    },
  }
}
