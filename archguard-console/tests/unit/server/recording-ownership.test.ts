import { describe, expect, it } from 'vitest'

import { _resetDbForTests, closeBrokerSession, listBrokerRecordingSessionsForPrincipal, listBrokerSessionsForPrincipal, registerBrokerSession } from '@/server/db'

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
})
