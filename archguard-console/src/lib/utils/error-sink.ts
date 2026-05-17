// src/lib/utils/error-sink.ts
//
// Single entry point for error reporting. In production, swap this for
// Sentry/Datadog/etc by replacing the body of `reportError`. The signature
// is intentionally narrow so callers don't depend on a specific provider.

export interface ErrorContext {
  /** Where in the app the error happened — e.g. "route:dashboard". */
  source?: string
  /** Tag, free-form. */
  tag?: string
  /** Extra structured data; do NOT include tokens or PII. */
  extra?: Record<string, unknown>
}

const subscribers = new Set<(err: Error, ctx: ErrorContext) => void>()

export function reportError(err: unknown, ctx: ErrorContext = {}): void {
  const error = err instanceof Error ? err : new Error(String(err))
  // Always log to the console so dev sees it without setup.
  // eslint-disable-next-line no-console
  console.error(`[error-sink]${ctx.source ? ' ' + ctx.source : ''}`, error, ctx)
  for (const sub of subscribers) {
    try {
      sub(error, ctx)
    } catch {
      // a broken subscriber must not break the app
    }
  }
}

/** Register an additional sink (e.g. Sentry). Returns an unsubscribe fn. */
export function subscribeErrors(
  fn: (err: Error, ctx: ErrorContext) => void,
): () => void {
  subscribers.add(fn)
  return () => subscribers.delete(fn)
}

/** For tests. */
export function _clearSubscribers(): void {
  subscribers.clear()
}
