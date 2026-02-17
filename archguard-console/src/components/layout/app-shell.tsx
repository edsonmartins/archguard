// src/components/layout/app-shell.tsx

import { useState } from 'react'
import { SidebarProvider, SidebarInset } from '@/components/ui/sidebar'
import { AppSidebar } from './app-sidebar'
import { Header } from './header'
import { CommandSearch } from './command-search'
import type { SessionUser } from '@/server/auth'

interface AppShellProps {
  user: SessionUser
  children: React.ReactNode
}

export function AppShell({ user, children }: AppShellProps) {
  const [searchOpen, setSearchOpen] = useState(false)

  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset>
        <Header user={user} onSearchOpen={() => setSearchOpen(true)} />
        <main className="flex-1 overflow-auto p-6">{children}</main>
      </SidebarInset>
      <CommandSearch open={searchOpen} onOpenChange={setSearchOpen} />
    </SidebarProvider>
  )
}
