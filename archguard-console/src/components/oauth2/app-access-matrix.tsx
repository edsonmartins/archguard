// src/components/oauth2/app-access-matrix.tsx
// Shows which groups (and their members) have access to an OAuth2 app
// via scope maps, grouped by tenant.

import { useMemo } from 'react'
import { Users, Building, Shield } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { extractTenantPrefix } from '@/lib/api/normalizers'
import { useGroups } from '@/lib/hooks/use-groups'
import { usePersons } from '@/lib/hooks/use-persons'
import type { OAuth2Client, Group, Person } from '@/lib/api/types/kanidm'

interface AccessEntry {
  groupId: string
  groupName: string
  scopes: string[]
  tenant: string | null
  members: { id: string; name: string; displayName: string }[]
}

export function AppAccessMatrix({ client }: { client: OAuth2Client }) {
  const { data: allGroups } = useGroups()
  const { data: allPersons } = usePersons()

  const accessEntries = useMemo((): AccessEntry[] => {
    if (!allGroups) return []

    const groupMap = new Map<string, Group>()
    for (const g of allGroups) {
      groupMap.set(g.id, g)
      groupMap.set(g.name, g)
    }

    // Merge scope maps and supplemental scope maps
    const allScopeMaps = [
      ...client.scopeMaps.map((sm) => ({ ...sm, type: 'primary' as const })),
      ...client.supplementalScopeMaps.map((sm) => ({ ...sm, type: 'supplemental' as const })),
    ]

    return allScopeMaps.map((sm) => {
      const group = groupMap.get(sm.groupId)
      const groupName = group?.name ?? sm.groupName || sm.groupId
      const tenant = extractTenantPrefix(groupName)

      // Find persons who are members of this group
      const members = (allPersons ?? [])
        .filter((p) =>
          p.groups.some((g) => g === groupName || g === sm.groupId),
        )
        .map((p) => ({
          id: p.id,
          name: p.username,
          displayName: p.displayName,
        }))

      return {
        groupId: sm.groupId,
        groupName,
        scopes: sm.scopes,
        tenant,
        members,
      }
    })
  }, [client, allGroups, allPersons])

  // Group entries by tenant
  const byTenant = useMemo(() => {
    const map = new Map<string, AccessEntry[]>()
    for (const entry of accessEntries) {
      const key = entry.tenant ?? '__system__'
      if (!map.has(key)) map.set(key, [])
      map.get(key)!.push(entry)
    }
    return map
  }, [accessEntries])

  if (accessEntries.length === 0) {
    return (
      <Card>
        <CardContent className="py-8 text-center text-muted-foreground">
          <Shield className="mx-auto mb-2 h-8 w-8" />
          <p>Nenhum scope map configurado. Adicione scope maps na aba "Scope Maps".</p>
        </CardContent>
      </Card>
    )
  }

  const totalUsers = new Set(accessEntries.flatMap((e) => e.members.map((m) => m.id))).size

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4 text-sm text-muted-foreground">
        <span>{accessEntries.length} grupo(s) com acesso</span>
        <span>{totalUsers} pessoa(s) total</span>
      </div>

      {Array.from(byTenant.entries()).map(([tenant, entries]) => (
        <Card key={tenant}>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              {tenant === '__system__' ? (
                <>
                  <Shield className="h-4 w-4" />
                  Grupos do Sistema
                </>
              ) : (
                <>
                  <Building className="h-4 w-4" />
                  Tenant: {tenant}
                </>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {entries.map((entry) => (
              <div
                key={entry.groupId}
                className="rounded-lg border p-3 space-y-2"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Users className="h-4 w-4 text-muted-foreground" />
                    <span className="font-medium text-sm">{entry.groupName}</span>
                  </div>
                  <div className="flex gap-1">
                    {entry.scopes.map((s) => (
                      <Badge key={s} variant="outline" className="text-xs">
                        {s}
                      </Badge>
                    ))}
                  </div>
                </div>
                {entry.members.length > 0 ? (
                  <div className="flex flex-wrap gap-1 pl-6">
                    {entry.members.slice(0, 10).map((m) => (
                      <Badge key={m.id} variant="secondary" className="text-xs">
                        {m.displayName || m.name}
                      </Badge>
                    ))}
                    {entry.members.length > 10 && (
                      <Badge variant="secondary" className="text-xs">
                        +{entry.members.length - 10} mais
                      </Badge>
                    )}
                  </div>
                ) : (
                  <p className="pl-6 text-xs text-muted-foreground">
                    Nenhum membro neste grupo
                  </p>
                )}
              </div>
            ))}
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
