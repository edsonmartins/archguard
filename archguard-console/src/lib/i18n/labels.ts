// Typed helpers for enum → human labels via i18n.
import type { TFunction } from 'i18next'

/** Translate enums.* keys; fall back to raw value if missing. */
export function enumLabel(
  t: TFunction,
  group:
    | 'ambiente'
    | 'tipo'
    | 'stack'
    | 'engine'
    | 'protocol'
    | 'warpgateKind'
    | 'personStatus'
    | 'credentialPolicy'
    | 'tokenExpiry',
  value: string | null | undefined,
): string {
  if (!value) return '—'
  const key = `enums.${group}.${value}`
  const translated = t(key)
  // i18next returns the key path when missing
  if (translated === key) return value
  return translated
}
