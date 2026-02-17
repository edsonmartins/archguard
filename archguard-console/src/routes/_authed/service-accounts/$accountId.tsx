// src/routes/_authed/service-accounts/$accountId.tsx

import { createFileRoute } from '@tanstack/react-router'
import { ServiceAccountDetailPage } from '@/components/service-account/sa-detail-page'

export const Route = createFileRoute('/_authed/service-accounts/$accountId')({
  component: ServiceAccountDetailPage,
})
