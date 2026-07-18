// src/components/dashboard/stats-cards.tsx

import { useTranslation } from 'react-i18next'
import { Users, UsersRound, KeyRound, ShieldCheck } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

interface StatsCardsProps {
  personsCount?: number
  groupsCount?: number
  oauth2Count?: number
  vaultOnline?: boolean
  isLoading: boolean
}

export function StatsCards({
  personsCount,
  groupsCount,
  oauth2Count,
  vaultOnline,
  isLoading,
}: StatsCardsProps) {
  const { t } = useTranslation()
  const cards = [
    {
      title: t('dashboard.stats.people'),
      value: personsCount ?? 0,
      icon: Users,
      description: t('dashboard.stats.peopleDesc'),
    },
    {
      title: t('dashboard.stats.groups'),
      value: groupsCount ?? 0,
      icon: UsersRound,
      description: t('dashboard.stats.groupsDesc'),
    },
    {
      title: t('dashboard.stats.oauth2'),
      value: oauth2Count ?? 0,
      icon: KeyRound,
      description: t('dashboard.stats.oauth2Desc'),
    },
    {
      title: t('dashboard.stats.vault'),
      value: vaultOnline ? t('common.online') : t('common.offline'),
      icon: ShieldCheck,
      description: vaultOnline
        ? t('dashboard.stats.vaultOnlineDesc')
        : t('dashboard.stats.vaultOfflineDesc'),
    },
  ]

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      {cards.map((card) => (
        <Card key={card.title}>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">{card.title}</CardTitle>
            <card.icon className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <>
                <Skeleton className="h-8 w-16" />
                <Skeleton className="mt-1 h-3 w-32" />
              </>
            ) : (
              <>
                <div className="text-2xl font-bold">{card.value}</div>
                <p className="text-xs text-muted-foreground">{card.description}</p>
              </>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
