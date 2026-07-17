// src/components/dashboard/system-health.tsx

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Activity } from 'lucide-react'
import { cn } from '@/lib/utils'

interface ServiceStatus {
  name: string
  status: 'ok' | 'error' | 'unreachable'
  version?: string
}

interface SystemHealthProps {
  services: ServiceStatus[]
  isLoading: boolean
}

const statusConfig: Record<
  string,
  {
    label: string
    variant: 'default' | 'secondary' | 'destructive' | 'outline'
    className?: string
  }
> = {
  ok: {
    label: 'Online',
    variant: 'outline',
    // Verde semântico (não usar primary/preto)
    className:
      'border-transparent bg-emerald-600 text-white hover:bg-emerald-600/90 dark:bg-emerald-600 dark:text-white',
  },
  error: {
    label: 'Erro',
    variant: 'destructive',
  },
  unreachable: {
    label: 'Indisponível',
    variant: 'secondary',
  },
}

export function SystemHealth({ services, isLoading }: SystemHealthProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <Activity className="h-4 w-4" />
          Status dos Serviços
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          {isLoading
            ? Array.from({ length: 2 }).map((_, i) => (
                <div key={i} className="flex items-center justify-between">
                  <span className="h-4 w-24 animate-pulse rounded bg-muted" />
                  <span className="h-5 w-16 animate-pulse rounded bg-muted" />
                </div>
              ))
            : services.map((service) => {
                const config =
                  statusConfig[service.status] ?? statusConfig.unreachable!
                return (
                  <div
                    key={service.name}
                    className="flex items-center justify-between"
                  >
                    <div>
                      <p className="text-sm font-medium">{service.name}</p>
                      {service.version && (
                        <p className="text-xs text-muted-foreground">
                          v{service.version}
                        </p>
                      )}
                    </div>
                    <Badge
                      variant={config.variant}
                      className={cn(config.className)}
                    >
                      {config.label}
                    </Badge>
                  </div>
                )
              })}
        </div>
      </CardContent>
    </Card>
  )
}
