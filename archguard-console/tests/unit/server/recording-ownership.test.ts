import { describe, expect, it } from 'vitest'

import { _resetDbForTests, closeBrokerSession, getLatestBrokerReconciliationRun, listBrokerLeaseInventory, listExpiredBrokerLeases, listBrokerRecordingSessionsForPrincipal, listBrokerSessionsForPrincipal, recordBrokerReconciliationRun, registerBrokerSession } from '@/server/db'

describe('recording ownership index', () => {
  it('keeps closed sessions eligible for their historical recording', () => {
    _resetDbForTests(`/tmp/archguard-recording-ownership-${process.pid}.sqlite`)
    registerBrokerSession('closed-session', undefined, 'alice')
    registerBrokerSession('open-session', undefined, 'alice')
    closeBrokerSession('closed-session')
    expect(listBrokerRecordingSessionsForPrincipal('alice')).toEqual(['closed-session', 'open-session'])
    expect(listBrokerSessionsForPrincipal('alice')).toEqual(['open-session'])
  })
  it('queries by principal without an open-session condition', () => {
    _resetDbForTests(`/tmp/archguard-recording-ownership-${process.pid}-empty.sqlite`)
    listBrokerRecordingSessionsForPrincipal('alice')
    expect(listBrokerRecordingSessionsForPrincipal('alice')).toEqual([])
  })

  it('keeps the lease inventory bound to session, tenant, target and expiry', () => {
    _resetDbForTests(`/tmp/archguard-recording-ownership-${process.pid}-inventory.sqlite`)
    registerBrokerSession('session-a', 'database/creds/role/lease-a', 'alice', 'tenant_a', 'db-a', '2020-01-01T00:00:00.000Z')
    registerBrokerSession('session-b', 'database/creds/role/lease-b', 'bob', 'tenant_b', 'db-b', '2099-01-01T00:00:00.000Z')
    const inventory = listBrokerLeaseInventory()
    expect(inventory).toHaveLength(2)
    expect(inventory).toContainEqual(expect.objectContaining({ session_id: 'session-a', lease_id: 'database/creds/role/lease-a', principal: 'alice', tenant: 'tenant_a', target: 'db-a' }))
    expect(inventory).toContainEqual(expect.objectContaining({ session_id: 'session-b', lease_id: 'database/creds/role/lease-b', principal: 'bob', tenant: 'tenant_b', target: 'db-b' }))
    expect(listExpiredBrokerLeases('2021-01-01T00:00:00.000Z').map((row) => row.lease_id)).toEqual(['database/creds/role/lease-a'])
  })

  it('persists the latest reconciliation cycle for restart-safe diagnostics', () => {
    _resetDbForTests(`/tmp/archguard-recording-ownership-${process.pid}-reconciliation.sqlite`)
    recordBrokerReconciliationRun('2026-09-11T13:00:00.000Z', { attempted: 3, closed: 2, failed: 1 }, 'one cleanup failed')
    expect(getLatestBrokerReconciliationRun()).toMatchObject({ attempted: 3, closed: 2, failed: 1, error: 'one cleanup failed' })
  })
})
