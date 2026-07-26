/**
 * PageHeader — padrão RecomX: título + Guia + ações da tela.
 * O Guia contextual também está no header global; aqui fica colado ao h1 da página.
 */

import type { ReactNode } from 'react'
import { GuideButton } from '@/components/shared/guide-button'
import { cn } from '@/lib/utils'

export type PageHeaderProps = {
  title: ReactNode
  description?: ReactNode
  /** Botões à direita (Nova conta, etc.) */
  actions?: ReactNode
  /** Mostrar Guia ao lado do título (default true) */
  showGuide?: boolean
  className?: string
  /** Classes do h1 */
  titleClassName?: string
}

export function PageHeader({
  title,
  description,
  actions,
  showGuide = true,
  className,
  titleClassName,
}: PageHeaderProps) {
  return (
    <div
      className={cn(
        'flex flex-wrap items-start justify-between gap-4',
        className,
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <h1
            className={cn(
              'text-2xl font-bold tracking-tight sm:text-3xl',
              titleClassName,
            )}
          >
            {title}
          </h1>
          {showGuide ? <GuideButton size="sm" variant="outline" /> : null}
        </div>
        {description ? (
          <div className="mt-1 max-w-2xl text-sm text-muted-foreground">
            {description}
          </div>
        ) : null}
      </div>
      {actions ? (
        <div className="flex flex-wrap items-center gap-2 shrink-0">
          {actions}
        </div>
      ) : null}
    </div>
  )
}
