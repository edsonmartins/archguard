// Proxy to host connector-agent (admin-first VPN materialization).
// Env: CONNECTOR_AGENT_URL, CONNECTOR_AGENT_TOKEN

import { logger } from './logger'
import { integrationFetch } from './http-integration-client'

const AGENT_URL = (
  process.env.CONNECTOR_AGENT_URL ||
  process.env.ARCHGATE_CONNECTOR_AGENT_URL ||
  ''
).replace(/\/$/, '')

const AGENT_TOKEN =
  process.env.CONNECTOR_AGENT_TOKEN ||
  process.env.ARCHGATE_CONNECTOR_AGENT_TOKEN ||
  ''

export function connectorAgentConfigured(): boolean {
  return Boolean(AGENT_URL && AGENT_TOKEN)
}

export function connectorAgentUrl(): string {
  return AGENT_URL
}

async function agentApi<T = unknown>(
  method: string,
  path: string,
  body?: unknown,
): Promise<{ status: number; data: T }> {
  if (!connectorAgentConfigured()) {
    throw new Error(
      'Connector agent não configurado (CONNECTOR_AGENT_URL + CONNECTOR_AGENT_TOKEN)',
    )
  }
  const res = await integrationFetch(`${AGENT_URL}${path}`, {
    method,
    integration: 'connector-agent',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${AGENT_TOKEN}`,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  const text = await res.text()
  let data: T
  try {
    data = text ? (JSON.parse(text) as T) : ({} as T)
  } catch {
    data = { raw: text } as T
  }
  if (res.status >= 400) {
    const err =
      (data as { error?: string })?.error ||
      `agent HTTP ${res.status}`
    logger.warn({ path, status: res.status, err }, 'connector-agent error')
    throw new Error(err)
  }
  return { status: res.status, data }
}

export async function agentHealth(): Promise<{
  status: string
  base?: string
}> {
  const { data } = await agentApi<{ status: string; base?: string }>(
    'GET',
    '/health',
  )
  return data
}

export async function agentListConnectors(): Promise<
  Array<{
    id: string
    stack: string
    conf_present: boolean
    unit?: { active?: boolean; enabled?: boolean; state?: string; unit?: string }
  }>
> {
  const { data } = await agentApi<{ connectors: Array<Record<string, unknown>> }>(
    'GET',
    '/v1/connectors',
  )
  return (data.connectors || []) as Array<{
    id: string
    stack: string
    conf_present: boolean
    unit?: { active?: boolean; enabled?: boolean; state?: string; unit?: string }
  }>
}

export async function agentPutConfig(
  id: string,
  stack: string,
  config: string,
): Promise<unknown> {
  const { data } = await agentApi('PUT', `/v1/connectors/${encodeURIComponent(id)}/config`, {
    stack,
    config,
  })
  return data
}

export async function agentStart(id: string, stack: string): Promise<unknown> {
  const { data } = await agentApi(
    'POST',
    `/v1/connectors/${encodeURIComponent(id)}/start`,
    { stack },
  )
  return data
}

export async function agentStop(id: string, stack: string): Promise<unknown> {
  const { data } = await agentApi(
    'POST',
    `/v1/connectors/${encodeURIComponent(id)}/stop`,
    { stack },
  )
  return data
}

export async function agentProbe(
  host: string,
  port: number,
): Promise<{ ok: boolean; detail?: string; host: string; port: number }> {
  const { data } = await agentApi<{
    ok: boolean
    detail?: string
    host: string
    port: number
  }>('POST', '/v1/probe', { host, port })
  return data
}

/** Build openfortivpn conf from structured fields (no password in SoT). */
export function buildFortiConf(input: {
  host: string
  port?: number
  username: string
  password: string
  trusted_cert?: string
}): string {
  const lines = [
    `host = ${input.host}`,
    `port = ${input.port ?? 443}`,
    `username = ${input.username}`,
    `password = ${input.password}`,
    'set-routes = 1',
    'set-dns = 0',
    'pppd-use-peerdns = 0',
  ]
  if (input.trusted_cert) {
    lines.push(`trusted-cert = ${input.trusted_cert}`)
  }
  return lines.join('\n') + '\n'
}
