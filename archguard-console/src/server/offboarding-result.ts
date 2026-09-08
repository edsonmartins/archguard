export function offboardingResult(steps: Array<{ component: string; ok: boolean }>) {
  const loginBlocked = steps.some((step) => step.component === 'idp' && step.ok)
  const complete = steps.length > 0 && steps.every((step) => step.ok)
  return { ok: complete, all_ok: complete, login_blocked: loginBlocked }
}
