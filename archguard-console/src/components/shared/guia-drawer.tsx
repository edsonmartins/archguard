// GuiaDrawer — painel lateral de guia contextual (padrão RecomX / Guia do Módulo).

import {
  AlertCircle,
  BookOpen,
  ChevronRight,
  Lightbulb,
  Link2,
  ListChecks,
  Settings2,
  Target,
} from 'lucide-react'
import { useRouterState } from '@tanstack/react-router'
import { getGuiaByRota, type GuiaModulo, type ItemFluxo } from '@/lib/guia/guia-modulos'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { ScrollArea } from '@/components/ui/scroll-area'
import { cn } from '@/lib/utils'

type GuiaDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Override path (tests); default = router pathname */
  path?: string
}

function SectionTitle({
  icon: Icon,
  title,
  className,
}: {
  icon: React.ComponentType<{ className?: string }>
  title: string
  className?: string
}) {
  return (
    <div className={cn('mb-2 flex items-center gap-2 text-xs font-semibold tracking-wide text-muted-foreground', className)}>
      <Icon className="h-3.5 w-3.5" />
      {title}
    </div>
  )
}

function GuiaContent({ guia }: { guia: GuiaModulo }) {
  return (
    <div className="space-y-5 pb-8">
      <div
        className="rounded-lg border p-4"
        style={{
          backgroundColor: `${guia.corAccent}14`,
          borderColor: `${guia.corAccent}33`,
        }}
      >
        <div className="flex items-start gap-3">
          <span className="text-3xl leading-none" aria-hidden>
            {guia.icone}
          </span>
          <div className="min-w-0">
            <h3
              className="text-lg font-bold leading-tight"
              style={{ color: guia.corAccent }}
            >
              {guia.titulo}
            </h3>
            <p className="text-xs text-muted-foreground">{guia.subtitulo}</p>
            <div className="mt-2 flex flex-wrap gap-1.5">
              <Badge variant="secondary">{guia.modulo}</Badge>
              <Badge variant="outline">{guia.fase}</Badge>
            </div>
          </div>
        </div>
      </div>

      <section>
        <SectionTitle icon={Target} title="OBJETIVO" />
        <p className="text-sm leading-relaxed">{guia.objetivo}</p>
      </section>

      <Separator />

      <section>
        <SectionTitle icon={AlertCircle} title="PROBLEMA QUE RESOLVE" />
        <p className="text-sm leading-relaxed text-muted-foreground">
          {guia.problema}
        </p>
      </section>

      <Separator />

      <section>
        <SectionTitle icon={Settings2} title="COMO FUNCIONA" />
        <ul className="space-y-2">
          {guia.comoFunciona.map((item) => (
            <li key={item} className="flex gap-2 text-sm leading-snug">
              <ChevronRight className="mt-0.5 h-3.5 w-3.5 shrink-0 text-violet-500" />
              <span>{item}</span>
            </li>
          ))}
        </ul>
      </section>

      <Separator />

      <section>
        <SectionTitle icon={ListChecks} title="FLUXO SUGERIDO" />
        <ol className="space-y-3">
          {guia.fluxoSugerido.map((passo: ItemFluxo) => (
            <li
              key={passo.passo}
              className="rounded-md border bg-card p-3 text-sm"
            >
              <div className="mb-1 flex items-center gap-2 font-medium">
                <Badge
                  variant="secondary"
                  className="h-5 min-w-5 justify-center rounded-full px-1.5"
                >
                  {passo.passo}
                </Badge>
                {passo.acao}
              </div>
              <p className="pl-7 text-muted-foreground leading-snug">
                {passo.descricao}
              </p>
            </li>
          ))}
        </ol>
      </section>

      <Separator />

      <section>
        <SectionTitle icon={Lightbulb} title="REGRAS-CHAVE" />
        <ul className="space-y-2">
          {guia.regrasChave.map((regra) => (
            <li
              key={regra}
              className="flex gap-2 rounded-md border bg-card p-2 text-xs leading-snug"
            >
              <Lightbulb className="mt-0.5 h-3.5 w-3.5 shrink-0 text-amber-500" />
              <span>{regra}</span>
            </li>
          ))}
        </ul>
      </section>

      {guia.integracoesFuturas && guia.integracoesFuturas.length > 0 ? (
        <>
          <Separator />
          <section>
            <SectionTitle icon={Link2} title="INTEGRAÇÕES / RELACIONADOS" />
            <div className="flex flex-wrap gap-1.5">
              {guia.integracoesFuturas.map((x) => (
                <Badge key={x} variant="outline">
                  {x}
                </Badge>
              ))}
            </div>
          </section>
        </>
      ) : null}
    </div>
  )
}

function GuiaVazio() {
  return (
    <div className="flex h-[50vh] flex-col items-center justify-center gap-3 px-6 text-center">
      <span className="text-4xl" aria-hidden>
        📖
      </span>
      <p className="font-semibold text-muted-foreground">Guia não disponível</p>
      <p className="max-w-xs text-sm text-muted-foreground">
        Esta rota ainda não tem guia contextual. Peça ao time de produto para
        incluir em <code className="text-xs">guia-modulos.ts</code>.
      </p>
    </div>
  )
}

export function GuiaDrawer({ open, onOpenChange, path }: GuiaDrawerProps) {
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const guia = getGuiaByRota(path ?? pathname)

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className="w-full gap-0 p-0 sm:max-w-lg! sm:w-[32rem]"
      >
        <SheetHeader className="border-b px-4 py-3 text-left">
          <SheetTitle className="flex items-center gap-2 text-base">
            <BookOpen className="h-5 w-5" />
            Guia do Módulo
          </SheetTitle>
        </SheetHeader>
        <ScrollArea className="h-[calc(100vh-4rem)] px-4 py-3">
          {guia ? <GuiaContent guia={guia} /> : <GuiaVazio />}
        </ScrollArea>
      </SheetContent>
    </Sheet>
  )
}
