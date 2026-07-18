import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  deleteSiteFn,
  exportAllSitesYamlFn,
  exportSiteYamlFn,
  getSiteFn,
  importSiteYamlFn,
  listSitesFn,
  getSitesBackendFn,
  seedDefaultSitesFn,
  upsertSiteFn,
} from '@/server/sites-fn'
import type { SiteInput } from '@/lib/api/types/site'

export const siteKeys = {
  all: ['sites'] as const,
  detail: (slug: string) => ['sites', slug] as const,
}

export function useSites() {
  return useQuery({
    queryKey: siteKeys.all,
    queryFn: () => listSitesFn(),
  })
}

export function useSitesBackend() {
  return useQuery({
    queryKey: ['sites', 'backend'] as const,
    queryFn: () => getSitesBackendFn(),
    staleTime: 60_000,
  })
}

export function useSite(slug: string) {
  return useQuery({
    queryKey: siteKeys.detail(slug),
    queryFn: () => getSiteFn({ data: { slug } }),
    enabled: !!slug,
  })
}

export function useUpsertSite() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: SiteInput) => upsertSiteFn({ data: input }),
    onSuccess: (site) => {
      void qc.invalidateQueries({ queryKey: siteKeys.all })
      void qc.invalidateQueries({ queryKey: siteKeys.detail(site.slug) })
    },
  })
}

export function useDeleteSite() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (slug: string) => deleteSiteFn({ data: { slug } }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: siteKeys.all })
    },
  })
}

function downloadText(filename: string, text: string) {
  const blob = new Blob([text], { type: 'text/yaml;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

export function useExportSiteYaml() {
  return useMutation({
    mutationFn: (slug: string) => exportSiteYamlFn({ data: { slug } }),
    onSuccess: (res) => downloadText(res.filename, res.yaml),
  })
}

export function useExportAllSitesYaml() {
  return useMutation({
    mutationFn: () => exportAllSitesYamlFn(),
    onSuccess: (res) => downloadText(res.filename, res.yaml),
  })
}

export function useImportSiteYaml() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (yaml: string) =>
      importSiteYamlFn({ data: { yaml, dry_run: false } }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: siteKeys.all })
    },
  })
}

/** Lab seed stubs — POST only, system:admin (not on GET list). */
export function useSeedDefaultSites() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => seedDefaultSitesFn(),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: siteKeys.all })
    },
  })
}
