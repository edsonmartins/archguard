/**
 * GuideButton — padrão RecomX.
 * Abre o GuiaDrawer contextual da rota atual (tela + negócio).
 */

import { useState } from 'react'
import { BookOpen } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { GuiaDrawer } from './guia-drawer'
import { cn } from '@/lib/utils'

export type GuideButtonProps = {
  /** Texto do botão; default i18n "Guia" */
  label?: string
  /** Só ícone (útil em toolbars densas) */
  iconOnly?: boolean
  className?: string
  size?: 'default' | 'sm' | 'lg' | 'icon'
  variant?:
    | 'default'
    | 'secondary'
    | 'outline'
    | 'ghost'
    | 'link'
    | 'destructive'
}

export function GuideButton({
  label,
  iconOnly = false,
  className,
  size = 'sm',
  variant = 'outline',
}: GuideButtonProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const text = label || t('guide.button', { defaultValue: 'Guia' })
  const tip = t('guide.tooltip', { defaultValue: 'Guia desta tela' })

  return (
    <>
      <TooltipProvider delayDuration={300}>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              type="button"
              variant={variant}
              size={iconOnly ? 'icon' : size}
              className={cn('gap-1.5 shrink-0', className)}
              onClick={() => setOpen(true)}
              aria-label={tip}
            >
              <BookOpen className="h-4 w-4" />
              {!iconOnly ? <span>{text}</span> : null}
            </Button>
          </TooltipTrigger>
          <TooltipContent side="bottom">{tip}</TooltipContent>
        </Tooltip>
      </TooltipProvider>
      <GuiaDrawer open={open} onOpenChange={setOpen} />
    </>
  )
}

export default GuideButton
