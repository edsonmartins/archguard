// Lista visual de clientes/sites ArchGate (conectividade + stack VPN)

import { useMemo, useRef, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  Building2,
  Plus,
  Search,
  Network,
  ShieldAlert,
  Download,
  Upload,
} from 'lucide-react'
import { toast } from 'sonner'
import { enumLabel } from '@/lib/i18n/labels'
import type { TFunction } from 'i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/shared/empty-state'
import { PermissionGate } from '@/components/shared/permission-gate'
import {
  useExportAllSitesYaml,
  useImportSiteYaml,
  useSeedDefaultSites,
  useSites,
  useSitesBackend,
} from '@/lib/hooks/use-sites'
import { usePermissions } from '@/lib/hooks/use-permissions'
import type { Site } from '@/lib/api/types/site'
import {
  checklistProgress,
  evaluateChecklist,
} from '@/lib/connector/checklist'

function stackBadge(stack: string, t: TFunction) {
  const variant =
    stack === 'a_confirmar'
      ? 'destructive'
      : stack === 'lab_overlay'
        ? 'secondary'
        : 'default'
  return (
    <Badge variant={variant as 'default' | 'secondary' | 'destructive'}>
      {enumLabel(t, 'stack', stack)}
    </Badge>
  )
}

function statusDots(site: Site) {
  return (
    <div className="flex gap-1.5" title="inventariado / connector / smoke">
      <span
        className={`h-2.5 w-2.5 rounded-full ${site.inventariado ? 'bg-emerald-500' : 'bg-muted-foreground/30'}`}
      />
      <span
        className={`h-2.5 w-2.5 rounded-full ${site.connector_deployed ? 'bg-emerald-500' : 'bg-muted-foreground/30'}`}
      />
      <span
        className={`h-2.5 w-2.5 rounded-full ${site.smoke_operador ? 'bg-emerald-500' : 'bg-muted-foreground/30'}`}
      />
    </div>
  )
}

export function SiteListPage() {
  const { t } = useTranslation()
  const { data: sites, isLoading, error } = useSites()
  const { data: backendInfo } = useSitesBackend()
  const { can } = usePermissions()
  const [q, setQ] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)
  const exportAll = useExportAllSitesYaml()
  const importYaml = useImportSiteYaml()
  const seedDefaults = useSeedDefaultSites()

  const filtered = useMemo(() => {
    if (!sites) return []
    const term = q.trim().toLowerCase()
    if (!term) return sites
    return sites.filter(
      (s) =>
        s.cliente.toLowerCase().includes(term) ||
        s.slug.toLowerCase().includes(term) ||
        s.stack.toLowerCase().includes(term) ||
        s.tenant_group.toLowerCase().includes(term),
    )
  }, [sites, q])

  const counts = useMemo(() => {
    const c: Record<string, number> = {}
    for (const s of sites || []) {
      c[s.stack] = (c[s.stack] || 0) + 1
    }
    return c
  }, [sites])

  if (error) {
    return (
      <div className="p-6 text-destructive flex items-center gap-2">
        <ShieldAlert className="h-5 w-5" />
        {(error as Error).message}
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
            <Building2 className="h-7 w-7 text-primary" />
            {t('sites.title')}
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            {t('sites.subtitle')}
            {backendInfo?.backend && (
              <span className="ml-2">
                <Badge variant="outline" className="font-mono text-[10px]">
                  SoT: {backendInfo.backend}
                </Badge>
              </span>
            )}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={exportAll.isPending}
            onClick={() =>
              exportAll.mutate(undefined, {
                onSuccess: () => toast.success(t('sites.exportYaml')),
                onError: (e) => toast.error((e as Error).message),
              })
            }
          >
            <Download className="h-4 w-4 mr-1" />
            {t('sites.exportYaml')}
          </Button>
          <PermissionGate require="sites:create">
            <input
              ref={fileRef}
              type="file"
              accept=".yaml,.yml,.json,text/yaml,application/json"
              className="hidden"
              onChange={(e) => {
                const f = e.target.files?.[0]
                if (!f) return
                const reader = new FileReader()
                reader.onload = () => {
                  const text = String(reader.result || '')
                  importYaml.mutate(text, {
                    onSuccess: (res) => {
                      if (res.allOk) {
                        toast.success(
                          `Importados ${res.imported} site(s)`,
                        )
                      } else {
                        toast.error(
                          res.results
                            .filter((r) => !r.ok)
                            .map((r) => r.error)
                            .join('; '),
                        )
                      }
                    },
                    onError: (err) => toast.error((err as Error).message),
                  })
                }
                reader.readAsText(f)
                e.target.value = ''
              }}
            />
            <Button
              variant="outline"
              size="sm"
              disabled={importYaml.isPending}
              onClick={() => fileRef.current?.click()}
            >
              <Upload className="h-4 w-4 mr-1" />
              {t('sites.importYaml')}
            </Button>
            <Button asChild size="sm">
              <Link to="/sites/create">
                <Plus className="h-4 w-4 mr-2" />
                {t('sites.new')}
              </Link>
            </Button>
          </PermissionGate>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {Object.entries(counts).map(([stack, n]) => (
          <Card key={stack}>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                {enumLabel(t, 'stack', stack)}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{n}</div>
            </CardContent>
          </Card>
        ))}
      </div>

      <div className="flex items-center gap-2">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            className="pl-9"
            placeholder={t('sites.search')}
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
        </div>
        <div className="text-xs text-muted-foreground flex items-center gap-2">
          <Network className="h-3.5 w-3.5" />
          {t('sites.statusLegend')}
        </div>
      </div>

      {isLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      ) : filtered.length === 0 ? (
        <div className="space-y-4">
          <EmptyState
            icon={Building2}
            title={t('sites.empty')}
            description={t('sites.emptyHint')}
          />
          {can('system:admin') && (
            <div className="flex justify-center">
              <Button
                variant="secondary"
                disabled={seedDefaults.isPending}
                onClick={() =>
                  seedDefaults.mutate(undefined, {
                    onSuccess: (r) =>
                      toast.success(
                        r.seeded > 0
                          ? `Seed: ${r.seeded} site(s)`
                          : 'Seed: OK',
                      ),
                    onError: (e) => toast.error((e as Error).message),
                  })
                }
              >
                {t('sites.seedLab')}
              </Button>
            </div>
          )}
        </div>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('sites.columns.client')}</TableHead>
                <TableHead>{t('sites.columns.slugTenant')}</TableHead>
                <TableHead>{t('sites.columns.environment')}</TableHead>
                <TableHead>{t('sites.columns.type')}</TableHead>
                <TableHead>{t('sites.columns.stack')}</TableHead>
                <TableHead>{t('sites.columns.targets')}</TableHead>
                <TableHead>{t('sites.columns.connector')}</TableHead>
                <TableHead>{t('sites.columns.status')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((site) => {
                const cp = checklistProgress(evaluateChecklist(site))
                return (
                <TableRow key={site.slug} className="cursor-pointer">
                  <TableCell>
                    <Link
                      to="/sites/$slug"
                      params={{ slug: site.slug }}
                      className="font-medium text-primary hover:underline"
                    >
                      {site.cliente}
                    </Link>
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    {site.slug}
                    <div className="text-muted-foreground">{site.tenant_group}</div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline">
                      {enumLabel(t, 'ambiente', site.ambiente)}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-xs">
                    {enumLabel(t, 'tipo', site.tipo)}
                  </TableCell>
                  <TableCell>{stackBadge(site.stack, t)}</TableCell>
                  <TableCell>{site.targets.length}</TableCell>
                  <TableCell>
                    <Link
                      to="/sites/$slug"
                      params={{ slug: site.slug }}
                      className="text-xs tabular-nums text-muted-foreground hover:text-primary"
                      title="Checklist connector"
                    >
                      {cp.pct}%
                    </Link>
                  </TableCell>
                  <TableCell>{statusDots(site)}</TableCell>
                </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}
