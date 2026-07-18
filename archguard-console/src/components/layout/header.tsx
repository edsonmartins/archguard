// src/components/layout/header.tsx

import { useMatches, Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Search } from 'lucide-react'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import { SidebarTrigger } from '@/components/ui/sidebar'
import { Separator } from '@/components/ui/separator'
import { UserMenu } from './user-menu'
import { LanguageSwitcher } from './language-switcher'
import type { SessionUser } from '@/server/auth'

const routeLabelKeys: Record<string, string> = {
  '/_authed/dashboard': 'nav.dashboard',
  '/_authed/identities': 'nav.identities',
  '/_authed/identities/': 'nav.identities',
  '/_authed/service-accounts': 'nav.serviceAccounts',
  '/_authed/groups': 'nav.groups',
  '/_authed/oauth2': 'nav.oauth2',
  '/_authed/vault': 'nav.vault',
  '/_authed/sites': 'nav.sites',
  '/_authed/gateways': 'nav.gateways',
  '/_authed/secrets': 'nav.secrets',
  '/_authed/platform': 'nav.platform',
  '/_authed/audit': 'nav.audit',
  '/_authed/recycle-bin': 'nav.recycleBin',
  '/_authed/settings': 'nav.settings',
}

interface HeaderProps {
  user: SessionUser
  onSearchOpen: () => void
}

export function Header({ user, onSearchOpen }: HeaderProps) {
  const matches = useMatches()
  const { t } = useTranslation()

  const breadcrumbs = matches
    .filter((m) => m.pathname !== '/' && m.routeId !== '__root__' && m.routeId !== '/_authed')
    .map((m) => {
      const key = routeLabelKeys[m.routeId]
      return {
        label: key
          ? t(key)
          : m.pathname.split('/').filter(Boolean).pop() ?? '',
        to: m.pathname,
      }
    })

  return (
    <header className="flex h-14 items-center gap-2 border-b px-4">
      <SidebarTrigger />
      <Separator orientation="vertical" className="mx-2 h-4" />
      <Breadcrumb className="flex-1">
        <BreadcrumbList>
          {breadcrumbs.map((crumb, i) => (
            <BreadcrumbItem key={crumb.to}>
              {i < breadcrumbs.length - 1 ? (
                <>
                  <BreadcrumbLink asChild>
                    <Link to={crumb.to}>{crumb.label}</Link>
                  </BreadcrumbLink>
                  <BreadcrumbSeparator />
                </>
              ) : (
                <BreadcrumbPage>{crumb.label}</BreadcrumbPage>
              )}
            </BreadcrumbItem>
          ))}
        </BreadcrumbList>
      </Breadcrumb>

      <LanguageSwitcher />

      <Button
        variant="outline"
        size="sm"
        className="gap-2 text-muted-foreground"
        onClick={onSearchOpen}
      >
        <Search className="h-4 w-4" />
        <span className="hidden sm:inline">{t('common.search')}</span>
        <kbd className="pointer-events-none hidden h-5 select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium opacity-100 sm:flex">
          <span className="text-xs">⌘</span>K
        </kbd>
      </Button>

      <UserMenu user={user} />
    </header>
  )
}
