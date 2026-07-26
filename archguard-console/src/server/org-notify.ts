// Optional webhook notify for dual-control pending checkouts (OCB-2 residual).
// Set ORG_CHECKOUT_WEBHOOK_URL (Slack incoming webhook or generic JSON POST).

import { logger } from './logger'

export type PendingCheckoutNotify = {
  checkout_id: string
  account_slug: string
  account_name: string
  principal: string
  reason: string
  ttl_seconds: number
  criticality?: string
}

/**
 * Fire-and-forget. Never throws to caller.
 * Slack-compatible: sends { text } ; generic: full JSON body.
 */
export async function notifyPendingCheckout(
  payload: PendingCheckoutNotify,
): Promise<void> {
  const url = (process.env.ORG_CHECKOUT_WEBHOOK_URL || '').trim()
  if (!url) return

  const text =
    `🔐 *Dual-control pendente*\n` +
    `• Conta: \`${payload.account_slug}\` (${payload.account_name})\n` +
    `• Solicitante: ${payload.principal}\n` +
    `• Motivo: ${payload.reason}\n` +
    `• TTL: ${payload.ttl_seconds}s\n` +
    `• Aprovar em: Contas da org → Aprovações pendentes`

  try {
    const isSlack = url.includes('hooks.slack.com')
    const body = isSlack
      ? { text }
      : {
          event: 'org_checkout.pending',
          text,
          ...payload,
        }
    const res = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    if (!res.ok) {
      logger.warn(
        { status: res.status, account: payload.account_slug },
        'org checkout webhook non-OK',
      )
    }
  } catch (e) {
    logger.warn(
      { err: String(e), account: payload.account_slug },
      'org checkout webhook failed',
    )
  }
}
