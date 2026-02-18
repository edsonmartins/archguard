// src/components/layout/app-sidebar.tsx

import { Link } from '@tanstack/react-router'
import {
  LayoutDashboard,
  Users,
  Bot,
  UsersRound,
  KeyRound,
  ShieldCheck,
  FileText,
  Settings,
  Trash2,
} from 'lucide-react'
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar'
import { usePermissions } from '@/lib/hooks/use-permissions'
import { TenantSwitcher } from '@/components/layout/tenant-switcher'
import type { Permission } from '@/lib/auth/permissions'

interface NavItem {
  label: string
  to: string
  icon: React.ComponentType<{ className?: string }>
  permission: Permission | null
}

const navigation: NavItem[] = [
  {
    label: 'Dashboard',
    to: '/dashboard',
    icon: LayoutDashboard,
    permission: null,
  },
  {
    label: 'Identidades',
    to: '/identities',
    icon: Users,
    permission: 'persons:read',
  },
  {
    label: 'Service Accounts',
    to: '/service-accounts',
    icon: Bot,
    permission: 'service_accounts:read',
  },
  {
    label: 'Grupos',
    to: '/groups',
    icon: UsersRound,
    permission: 'groups:read',
  },
  {
    label: 'OAuth2 / SSO',
    to: '/oauth2',
    icon: KeyRound,
    permission: 'oauth2:read',
  },
  {
    label: 'Vault',
    to: '/vault',
    icon: ShieldCheck,
    permission: 'vault:read',
  },
  {
    label: 'Auditoria',
    to: '/audit',
    icon: FileText,
    permission: 'audit:read',
  },
  {
    label: 'Lixeira',
    to: '/recycle-bin',
    icon: Trash2,
    permission: 'system:admin',
  },
  {
    label: 'Configurações',
    to: '/settings',
    icon: Settings,
    permission: 'settings:read',
  },
]

export function AppSidebar() {
  const { can } = usePermissions()

  const filteredNav = navigation.filter(
    (item) => !item.permission || can(item.permission),
  )

  return (
    <Sidebar>
      <SidebarHeader className="border-b px-4 py-3">
        <Link to="/dashboard" className="flex items-center gap-2">
          <ShieldCheck className="h-6 w-6 text-primary" />
          <div>
            <h1 className="text-base font-bold leading-none">ArchGuard</h1>
            <p className="text-xs text-muted-foreground">Console</p>
          </div>
        </Link>
      </SidebarHeader>
      <SidebarContent>
        <TenantSwitcher />
        <SidebarGroup>
          <SidebarGroupLabel>Navegação</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {filteredNav.map((item) => (
                <SidebarMenuItem key={item.to}>
                  <SidebarMenuButton asChild>
                    <Link
                      to={item.to as string}
                      activeProps={{ className: 'bg-accent font-medium' }}
                    >
                      <item.icon className="h-4 w-4" />
                      <span>{item.label}</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
    </Sidebar>
  )
}
