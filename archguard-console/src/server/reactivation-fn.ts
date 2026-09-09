import { createServerFn } from '@tanstack/react-start'
import { z } from 'zod'
import { requireSession, requireAnyPerm, sessionActor } from './session-guard'
import { recordActivity } from './activity-log'

export const reactivatePersonFn = createServerFn({ method: 'POST' })
  .inputValidator((input: unknown) => z.object({
    username: z.string().trim().min(1).max(128),
  }).parse(input))
  .handler(async ({ data }) => {
    const session = requireSession()
    requireAnyPerm(session, ['system:admin'])
    // Require evidence that this IdP supplies original authentication time.
    if (!Number.isFinite(session.authTime)) {
      throw new Error('Reativação indisponível: faça novo login com auth_time validado pelo IdP')
    }
    const { isPrincipalRevoked, reactivatePrincipal } = await import('./principal-revocation')
    if (!isPrincipalRevoked(data.username)) throw new Error('Usuário não está bloqueado no console')
    const { listBrokerSessionsForPrincipal } = await import('./db')
    if (listBrokerSessionsForPrincipal(data.username).length) {
      throw new Error('Conclua o encerramento das sessões pendentes antes de reativar')
    }
    const { enableArchGuardUser } = await import('./idp/archguard')
    const actor = sessionActor(session)
    try {
      const result = await enableArchGuardUser(data.username)
      if (!result.ok) throw new Error(result.detail)
      reactivatePrincipal(data.username)
      recordActivity('POST', `/archgate/persons/${encodeURIComponent(data.username)}/reactivate`, actor, 'success')
      return { ok: true, message: 'Login reativado. Exija novo login pelo console; revise as concessões de acesso.' }
    } catch (error) {
      recordActivity('POST', `/archgate/persons/${encodeURIComponent(data.username)}/reactivate`, actor, 'error', 'Reactivation failed')
      throw error
    }
  })
