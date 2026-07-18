// src/components/shared/not-found.tsx

import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { FileQuestion, Home, ArrowLeft } from 'lucide-react'
import { Button } from '@/components/ui/button'

export function NotFound() {
  const { t } = useTranslation()
  return (
    <div className="flex min-h-[50vh] items-center justify-center p-8">
      <div className="text-center">
        <FileQuestion className="mx-auto mb-4 h-16 w-16 text-muted-foreground/50" />
        <h2 className="mb-2 text-2xl font-bold">{t('common.notFound')}</h2>
        <p className="mb-6 text-muted-foreground">{t('errors.notFound')}</p>
        <div className="flex justify-center gap-2">
          <Button variant="outline" onClick={() => window.history.back()}>
            <ArrowLeft className="mr-2 h-4 w-4" />
            {t('common.back')}
          </Button>
          <Button asChild>
            <Link to="/dashboard">
              <Home className="mr-2 h-4 w-4" />
              {t('nav.dashboard')}
            </Link>
          </Button>
        </div>
      </div>
    </div>
  )
}
