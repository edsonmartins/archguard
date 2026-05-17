import { describe, it, expect } from 'vitest'
import ptBR from '@/lib/i18n/pt-BR.json'
import en from '@/lib/i18n/en.json'

type Bundle = Record<string, unknown>

function flatten(obj: Bundle, prefix = ''): string[] {
  const keys: string[] = []
  for (const [key, value] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      keys.push(...flatten(value as Bundle, path))
    } else {
      keys.push(path)
    }
  }
  return keys.sort()
}

describe('i18n parity (pt-BR ↔ en)', () => {
  const ptKeys = flatten(ptBR as Bundle)
  const enKeys = flatten(en as Bundle)

  it('en is not missing keys defined in pt-BR', () => {
    const missing = ptKeys.filter((k) => !enKeys.includes(k))
    expect(missing, `missing in en: ${missing.join(', ')}`).toEqual([])
  })

  it('pt-BR is not missing keys defined in en', () => {
    const missing = enKeys.filter((k) => !ptKeys.includes(k))
    expect(missing, `missing in pt-BR: ${missing.join(', ')}`).toEqual([])
  })

  it('every key has a non-empty string value in both bundles', () => {
    const empties: string[] = []
    function walk(b: Bundle, prefix = ''): void {
      for (const [k, v] of Object.entries(b)) {
        const path = prefix ? `${prefix}.${k}` : k
        if (v && typeof v === 'object' && !Array.isArray(v)) walk(v as Bundle, path)
        else if (typeof v !== 'string' || v.trim() === '') empties.push(path)
      }
    }
    walk(ptBR as Bundle, 'pt-BR')
    walk(en as Bundle, 'en')
    expect(empties, `empty values: ${empties.join(', ')}`).toEqual([])
  })
})
