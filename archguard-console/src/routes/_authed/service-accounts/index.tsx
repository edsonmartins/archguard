// src/routes/_authed/service-accounts/index.tsx

import { createFileRoute } from '@tanstack/react-router'
import { ServiceAccountListPage } from '@/components/service-account/sa-list-page'

export const Route = createFileRoute('/_authed/service-accounts/')({
  component: ServiceAccountListPage,
})
