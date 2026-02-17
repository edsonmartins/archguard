// src/routes/_authed/vault.tsx

import { createFileRoute } from '@tanstack/react-router'
import { VaultDashboard } from '@/components/vault/vault-dashboard'

export const Route = createFileRoute('/_authed/vault')({
  component: VaultDashboard,
})
