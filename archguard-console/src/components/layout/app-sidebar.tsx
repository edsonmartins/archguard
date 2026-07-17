// src/components/layout/app-sidebar.tsx
// Control plane navigation (AWS Console–style modules)

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
  Building2,
  Server,
  KeyRound as KeyRoundIcon,
  Cloud,
  Gauge,
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

interface NavGroup {
  label: string
  items: NavItem[]
}

const navGroups: NavGroup[] = [
  {
    label: 'Visão geral',
    items: [
      {
        label: 'Dashboard',
        to: '/dashboard',
        icon: LayoutDashboard,
        permission: null,
      },
      {
        label: 'Plataforma',
        to: '/platform',
        icon: Gauge,
        permission: 'sites:read',
      },
    ],
  },
  {
    label: 'Identidade',
    items: [
      {
        label: 'Identidades',
        to: '/identities',
        icon: Users,
        permission: 'persons:read',
      },
      {
        label: 'Grupos',
        to: '/groups',
        icon: UsersRound,
        permission: 'groups:read',
      },
      {
        label: 'Service Accounts',
        to: '/service-accounts',
        icon: Bot,
        permission: 'service_accounts:read',
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
    ],
  },
  {
    label: 'Acesso (ArchGate)',
    items: [
      {
        label: 'Clientes / Sites',
        to: '/sites',
        icon: Building2,
        permission: 'sites:read',
      },
      {
        label: 'Gateways',
        to: '/gateways',
        icon: Server,
        permission: 'gateways:read',
      },
      {
        label: 'Segredos',
        to: '/secrets',
        icon: KeyRoundIcon,
        permission: 'secrets:read',
      },
      {
        label: 'Mentors Axis',
        to: '/integrations/mentors-axis',
        icon: Cloud,
        permission: 'sites:update',
      },
    ],
  },
  {
    label: 'Governança',
    items: [
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
    ],
  },
]

export function AppSidebar() {
  const { can } = usePermissions()

  return (
    <Sidebar>
      <SidebarHeader className="border-b px-4 py-3">
        <Link to="/dashboard" className="flex items-center gap-2">
          <ShieldCheck className="h-6 w-6 text-primary" />
          <div>
            <span className="text-base font-bold leading-none">ArchGate</span>
            <p className="text-xs text-muted-foreground">Admin unificado</p>
          </div>
        </Link>
      </SidebarHeader>
      <SidebarContent>
        <TenantSwitcher />
        {navGroups.map((group) => {
          const items = group.items.filter(
            (item) => !item.permission || can(item.permission),
          )
          if (items.length === 0) return null
          return (
            <SidebarGroup key={group.label}>
              <SidebarGroupLabel>{group.label}</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  {items.map((item) => (
                    <SidebarMenuItem key={item.to}>
                      <SidebarMenuButton asChild>
                        <Link to={item.to}>
                          <item.icon className="h-4 w-4" />
                          <span>{item.label}</span>
                        </Link>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  ))}
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          )
        })}
      </SidebarContent>
    </Sidebar>
  )
}
