// src/lib/i18n/config.ts
import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import ptBR from './pt-BR.json'
import en from './en.json'

export const SUPPORTED_LOCALES = ['pt-BR', 'en'] as const
export type AppLocale = (typeof SUPPORTED_LOCALES)[number]

export const LOCALE_STORAGE_KEY = 'archguard.locale'

export function isAppLocale(value: string | null | undefined): value is AppLocale {
  return value === 'pt-BR' || value === 'en'
}

export function detectInitialLocale(): AppLocale {
  if (typeof window !== 'undefined') {
    try {
      const stored = localStorage.getItem(LOCALE_STORAGE_KEY)
      if (isAppLocale(stored)) return stored
    } catch {
      /* private mode */
    }
    const nav = (navigator.language || '').toLowerCase()
    if (nav.startsWith('pt')) return 'pt-BR'
  }
  return 'pt-BR'
}

function syncDocumentLang(lng: string) {
  if (typeof document !== 'undefined') {
    document.documentElement.lang = lng === 'en' ? 'en' : 'pt-BR'
  }
}

if (!i18n.isInitialized) {
  i18n.use(initReactI18next).init({
    resources: {
      'pt-BR': { translation: ptBR },
      en: { translation: en },
    },
    lng: detectInitialLocale(),
    fallbackLng: 'en',
    supportedLngs: [...SUPPORTED_LOCALES],
    interpolation: { escapeValue: false },
    returnNull: false,
  })
  syncDocumentLang(i18n.language)
  i18n.on('languageChanged', (lng) => {
    syncDocumentLang(lng)
    if (typeof window !== 'undefined') {
      try {
        if (isAppLocale(lng)) localStorage.setItem(LOCALE_STORAGE_KEY, lng)
      } catch {
        /* ignore */
      }
    }
  })
}

export async function setAppLocale(locale: AppLocale): Promise<void> {
  await i18n.changeLanguage(locale)
}

export default i18n
