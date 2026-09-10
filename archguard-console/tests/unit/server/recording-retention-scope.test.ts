import { beforeEach, expect, it, vi } from 'vitest'
const m = vi.hoisted(() => ({ session: vi.fn(), perms: vi.fn(), recordings: vi.fn(), broker: vi.fn(), retention: vi.fn(), set: vi.fn(), audit: vi.fn() }))
vi.mock('@tanstack/react-router', () => ({ createFileRoute: () => (config: unknown) => config }))
vi.mock('@/server/operator-session', () => ({ resolveOperatorSession: m.session }))
vi.mock('@/server/session-guard', () => ({ hasAnyPerm: m.perms, requireAnyPerm: vi.fn(), sessionActor: () => 'alice' }))
vi.mock('@/server/rustguac-proxy', () => ({ listRustGuacRecordings: m.recordings, getRecordingRetention: m.retention, setRecordingRetention: m.set }))
vi.mock('@/server/db', () => ({ getBrokerSession: m.broker }))
vi.mock('@/server/activity-log', () => ({ recordActivity: m.audit }))
vi.mock('@/server/unified-cors', () => ({ unifiedCorsHeaders: () => ({}) }))
import { Route } from '@/routes/api/unified/v1/recordings.$name.retention'

beforeEach(() => vi.resetAllMocks())

const post = (name: string, body: unknown) => {
  const handler = (Route as unknown as { server: { handlers: { POST: Function } } }).server.handlers.POST
  return handler({ request: new Request('http://test', { method: 'POST', body: JSON.stringify(body) }), params: { name } })
}
it('refuses retention for a UUID not present in RustGuac', async () => {
  m.session.mockResolvedValue({ user: { name: 'alice' } }); m.perms.mockReturnValue(false); m.recordings.mockResolvedValue([])
  const response = await post('11111111-1111-4111-8111-111111111111.guac', { legal_hold: true })
  expect(response.status).toBe(404); expect(m.broker).not.toHaveBeenCalled(); expect(m.set).not.toHaveBeenCalled()
})
it('refuses tenant retention when historical ownership is absent', async () => {
  m.session.mockResolvedValue({ user: { name: 'alice' } }); m.perms.mockReturnValue(false)
  m.recordings.mockResolvedValue([{ name: '11111111-1111-4111-8111-111111111111.guac' }]); m.broker.mockReturnValue(undefined)
  const response = await post('11111111-1111-4111-8111-111111111111.guac', { legal_hold: true })
  expect(response.status).toBe(404); expect(m.set).not.toHaveBeenCalled()
})
it('allows platform administration after validating the recording exists', async () => {
  m.session.mockResolvedValue({ user: { name: 'admin' } }); m.perms.mockReturnValue(true)
  m.recordings.mockResolvedValue([{ name: '11111111-1111-4111-8111-111111111111.guac' }]); m.retention.mockReturnValue({ retain_until: null }); m.set.mockReturnValue({ legal_hold: true })
  const response = await post('11111111-1111-4111-8111-111111111111.guac', { legal_hold: true })
  expect(response.status).toBe(200); expect(m.broker).not.toHaveBeenCalled(); expect(m.set).toHaveBeenCalled()
})
