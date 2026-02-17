// src/routes/_authed/service-accounts/create.tsx

import { createFileRoute } from '@tanstack/react-router'
import { ServiceAccountCreatePage } from '@/components/service-account/sa-create-page'

export const Route = createFileRoute('/_authed/service-accounts/create')({
  component: ServiceAccountCreatePage,
})
