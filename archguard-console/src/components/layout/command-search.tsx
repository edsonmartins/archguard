// src/components/layout/command-search.tsx

import { useState, useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  Users,
  UsersRound,
  KeyRound,
  Plus,
  LayoutDashboard,
  FileText,
  Settings,
} from 'lucide-react'
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command'

interface CommandSearchProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function CommandSearch({ open, onOpenChange }: CommandSearchProps) {
  const navigate = useNavigate()

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        onOpenChange(!open)
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [open, onOpenChange])

  const goTo = (to: string) => {
    onOpenChange(false)
    navigate({ to: to as string })
  }

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange}>
      <CommandInput placeholder="Buscar pessoas, grupos, apps..." />
      <CommandList>
        <CommandEmpty>Nenhum resultado encontrado.</CommandEmpty>
        <CommandGroup heading="Páginas">
          <CommandItem onSelect={() => goTo('/dashboard')}>
            <LayoutDashboard className="mr-2 h-4 w-4" />
            Dashboard
          </CommandItem>
          <CommandItem onSelect={() => goTo('/identities')}>
            <Users className="mr-2 h-4 w-4" />
            Identidades
          </CommandItem>
          <CommandItem onSelect={() => goTo('/groups')}>
            <UsersRound className="mr-2 h-4 w-4" />
            Grupos
          </CommandItem>
          <CommandItem onSelect={() => goTo('/oauth2')}>
            <KeyRound className="mr-2 h-4 w-4" />
            OAuth2 / SSO
          </CommandItem>
          <CommandItem onSelect={() => goTo('/audit')}>
            <FileText className="mr-2 h-4 w-4" />
            Auditoria
          </CommandItem>
          <CommandItem onSelect={() => goTo('/settings')}>
            <Settings className="mr-2 h-4 w-4" />
            Configurações
          </CommandItem>
        </CommandGroup>
        <CommandSeparator />
        <CommandGroup heading="Ações rápidas">
          <CommandItem onSelect={() => goTo('/identities/create')}>
            <Plus className="mr-2 h-4 w-4" />
            Nova Pessoa
          </CommandItem>
          <CommandItem onSelect={() => goTo('/groups/create')}>
            <Plus className="mr-2 h-4 w-4" />
            Novo Grupo
          </CommandItem>
          <CommandItem onSelect={() => goTo('/oauth2/create')}>
            <Plus className="mr-2 h-4 w-4" />
            Novo Client OAuth2
          </CommandItem>
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  )
}
