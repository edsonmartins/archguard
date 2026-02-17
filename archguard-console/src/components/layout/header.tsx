// src/components/layout/header.tsx

import { useMatches, Link } from '@tanstack/react-router'
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
import type { SessionUser } from '@/server/auth'

const routeLabels: Record<string, string> = {
  '/_authed/dashboard': 'Dashboard',
  '/_authed/identities': 'Identidades',
  '/_authed/identities/': 'Identidades',
  '/_authed/service-accounts': 'Service Accounts',
  '/_authed/groups': 'Grupos',
  '/_authed/oauth2': 'OAuth2 / SSO',
  '/_authed/vault': 'Vault',
  '/_authed/audit': 'Auditoria',
  '/_authed/settings': 'Configurações',
}

interface HeaderProps {
  user: SessionUser
  onSearchOpen: () => void
}

export function Header({ user, onSearchOpen }: HeaderProps) {
  const matches = useMatches()

  const breadcrumbs = matches
    .filter((m) => m.pathname !== '/' && m.routeId !== '__root__' && m.routeId !== '/_authed')
    .map((m) => ({
      label:
        routeLabels[m.routeId] ??
        m.pathname.split('/').filter(Boolean).pop() ??
        '',
      to: m.pathname,
    }))

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

      <Button
        variant="outline"
        size="sm"
        className="gap-2 text-muted-foreground"
        onClick={onSearchOpen}
      >
        <Search className="h-4 w-4" />
        <span className="hidden sm:inline">Buscar...</span>
        <kbd className="pointer-events-none hidden h-5 select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium opacity-100 sm:flex">
          <span className="text-xs">⌘</span>K
        </kbd>
      </Button>

      <UserMenu user={user} />
    </header>
  )
}
