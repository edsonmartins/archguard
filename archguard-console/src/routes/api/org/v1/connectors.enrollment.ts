// POST /api/org/v1/connectors/enrollment — issue or consume connector enrollment.

import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { requireAnyPerm, requireSession, sessionActor } from '@/server/session-guard'
import { issueConnectorEnrollment, consumeConnectorEnrollment, registerConnectorCertificate, markConnectorCertificatesRevoked } from '@/server/connector-enrollment'
import { signConnectorCertificate, revokeConnectorCertificate } from '@/server/openbao-proxy'

const issueSchema = z.object({
  action: z.literal('issue'),
  site_slug: z.string().min(1).max(64),
  connector_id: z.string().min(1).max(128),
  ttl_seconds: z.number().int().min(60).max(3600).optional(),
})
const consumeSchema = z.object({ action: z.literal('consume'), token: z.string().min(20).max(256) })
const signSchema = z.object({
  action: z.literal('sign'),
  token: z.string().min(20).max(256),
  csr: z.string().min(100).max(32_000),
})
const revokeSchema = z.object({ action: z.literal('revoke'), connector_id: z.string().min(1).max(128), serial_number: z.string().min(1).max(256) })

export const Route = createFileRoute('/api/org/v1/connectors/enrollment')({
  server: {
    handlers: {
      POST: async ({ request }) => {
        try {
          const body = await request.json().catch(() => ({}))
          if ((body as { action?: string }).action === 'consume') {
            const data = consumeSchema.parse(body)
            const enrollment = consumeConnectorEnrollment(data.token)
            if (!enrollment) return Response.json({ error: 'invalid or expired enrollment token' }, { status: 401 })
            return Response.json({ enrollment })
          }
          if ((body as { action?: string }).action === 'sign') {
            const data = signSchema.parse(body)
            const enrollment = consumeConnectorEnrollment(data.token)
            if (!enrollment) return Response.json({ error: 'invalid or expired enrollment token' }, { status: 401 })
            const certificate = await signConnectorCertificate(data.csr)
            registerConnectorCertificate({
              serial_number: certificate.serial_number,
              connector_id: enrollment.connector_id,
              site_slug: enrollment.site_slug,
            })
            return Response.json({ enrollment, certificate })
          }
          if ((body as { action?: string }).action === 'revoke') {
            const session = requireSession()
            requireAnyPerm(session, ['sites:update'], 'sites:update')
            const data = revokeSchema.parse(body)
            await revokeConnectorCertificate(data.serial_number)
            const revoked = markConnectorCertificatesRevoked(data.connector_id)
            return Response.json({ revoked })
          }
          const session = requireSession()
          requireAnyPerm(session, ['sites:update'], 'sites:update')
          const data = issueSchema.parse(body)
          const issued = issueConnectorEnrollment({
            ...data,
            created_by: sessionActor(session),
          })
          return Response.json(issued, { status: 201 })
        } catch (e) {
          const msg = (e as Error).message || 'invalid request'
          const status = msg.includes('Forbidden') ? 403 : msg.includes('Unauthorized') ? 401 : 400
          return Response.json({ error: msg }, { status })
        }
      },
    },
  },
})
