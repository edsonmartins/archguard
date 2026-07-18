// Compact language toggle (pt-BR / en)

import { Languages } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  setAppLocale,
  type AppLocale,
  isAppLocale,
} from '@/lib/i18n/config'

export function LanguageSwitcher({
  className,
  compact = true,
}: {
  className?: string
  compact?: boolean
}) {
  const { t, i18n } = useTranslation()
  const current: AppLocale = isAppLocale(i18n.language)
    ? i18n.language
    : i18n.language?.startsWith('pt')
      ? 'pt-BR'
      : 'en'

  return (
    <Select
      value={current}
      onValueChange={(v) => {
        if (isAppLocale(v)) void setAppLocale(v)
      }}
    >
      <SelectTrigger
        aria-label={t('language.switch')}
        className={
          className ||
          (compact ? 'h-8 w-[7.5rem] text-xs' : 'w-full max-w-xs')
        }
      >
        <Languages className="h-3.5 w-3.5 mr-1.5 shrink-0 opacity-70" />
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="pt-BR">{t('language.ptBR')}</SelectItem>
        <SelectItem value="en">{t('language.en')}</SelectItem>
      </SelectContent>
    </Select>
  )
}
