// Control plane navigation (AWS Console–style modules)

import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
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
  labelKey: string
  to: string
  icon: React.ComponentType<{ className?: string }>
  permission: Permission | null
}

interface NavGroup {
  labelKey: string
  items: NavItem[]
}

const navGroups: NavGroup[] = [
  {
    labelKey: 'nav.overview',
    items: [
      {
        labelKey: 'nav.dashboard',
        to: '/dashboard',
        icon: LayoutDashboard,
        permission: null,
      },
      {
        labelKey: 'nav.platform',
        to: '/platform',
        icon: Gauge,
        permission: 'sites:read',
      },
    ],
  },
  {
    labelKey: 'nav.identity',
    items: [
      {
        labelKey: 'nav.identities',
        to: '/identities',
        icon: Users,
        permission: 'persons:read',
      },
      {
        labelKey: 'nav.groups',
        to: '/groups',
        icon: UsersRound,
        permission: 'groups:read',
      },
      {
        labelKey: 'nav.serviceAccounts',
        to: '/service-accounts',
        icon: Bot,
        permission: 'service_accounts:read',
      },
      {
        labelKey: 'nav.oauth2',
        to: '/oauth2',
        icon: KeyRound,
        permission: 'oauth2:read',
      },
      {
        labelKey: 'nav.vault',
        to: '/vault',
        icon: ShieldCheck,
        permission: 'vault:read',
      },
    ],
  },
  {
    labelKey: 'nav.access',
    items: [
      {
        labelKey: 'nav.sites',
        to: '/sites',
        icon: Building2,
        permission: 'sites:read',
      },
      {
        labelKey: 'nav.gateways',
        to: '/gateways',
        icon: Server,
        permission: 'gateways:read',
      },
      {
        labelKey: 'nav.secrets',
        to: '/secrets',
        icon: KeyRoundIcon,
        permission: 'secrets:read',
      },
      {
        labelKey: 'nav.mentorsAxis',
        to: '/integrations/mentors-axis',
        icon: Cloud,
        permission: 'sites:update',
      },
    ],
  },
  {
    labelKey: 'nav.governance',
    items: [
      {
        labelKey: 'nav.audit',
        to: '/audit',
        icon: FileText,
        permission: 'audit:read',
      },
      {
        labelKey: 'nav.recycleBin',
        to: '/recycle-bin',
        icon: Trash2,
        permission: 'system:admin',
      },
      {
        labelKey: 'nav.settings',
        to: '/settings',
        icon: Settings,
        permission: 'settings:read',
      },
    ],
  },
]

export function AppSidebar() {
  const { t } = useTranslation()
  const { can } = usePermissions()

  return (
    <Sidebar>
      <SidebarHeader className="border-b px-4 py-3">
        <Link to="/dashboard" className="flex items-center gap-2">
          <ShieldCheck className="h-6 w-6 text-primary" />
          <div>
            <span className="text-base font-bold leading-none">ArchGate</span>
            <p className="text-xs text-muted-foreground">Admin</p>
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
            <SidebarGroup key={group.labelKey}>
              <SidebarGroupLabel>{t(group.labelKey)}</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  {items.map((item) => (
                    <SidebarMenuItem key={item.to}>
                      <SidebarMenuButton asChild>
                        <Link to={item.to}>
                          <item.icon className="h-4 w-4" />
                          <span>{t(item.labelKey)}</span>
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
