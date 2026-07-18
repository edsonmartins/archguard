import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import {
  createConnection,
  deleteConnection,
  guacamoleConfigured,
  listConnections,
} from './guacamole-proxy'
import { requireAnyPerm, requireSession } from './session-guard'
import {
  filterByTargetNames,
  gatewayTenantScope,
} from './gateway-scope'

const READ_PERMS = [
  'gateways:read',
  'gateways:manage',
  'sites:update',
] as const

const MANAGE_PERMS = ['gateways:manage'] as const

export const getGuacamoleStatusFn = createServerFn({ method: 'GET' }).handler(
  async () => {
    requireSession()
    return {
      configured: guacamoleConfigured(),
      url: process.env.GUACAMOLE_PUBLIC_URL || 'https://guac.archgate.com.br',
    }
  },
)

export const listGuacamoleConnectionsFn = createServerFn({
  method: 'GET',
}).handler(async () => {
  const s = requireSession()
  requireAnyPerm(s, [...READ_PERMS], 'gateways:read')
  const scope = await gatewayTenantScope(s)
  const conns = await listConnections()
  return filterByTargetNames(conns, scope)
})

export const createGuacamoleConnectionFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z
      .object({
        name: z.string().min(1).max(128),
        protocol: z.enum(['ssh', 'rdp', 'vnc']),
        hostname: z.string().min(1),
        port: z.number().int().positive(),
        username: z.string().optional(),
        password: z.string().optional(),
      })
      .safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, [...MANAGE_PERMS], 'gateways:manage')
    return createConnection(data)
  })

export const deleteGuacamoleConnectionFn = createServerFn({ method: 'POST' })
  .inputValidator((data: unknown) => {
    const r = z.object({ id: z.string().min(1) }).safeParse(data)
    if (!r.success) throw new Error(r.error.message)
    return r.data
  })
  .handler(async ({ data }) => {
    const s = requireSession()
    requireAnyPerm(s, [...MANAGE_PERMS], 'gateways:manage')
    await deleteConnection(data.id)
    return { ok: true }
  })
