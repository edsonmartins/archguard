// src/routes/_authed/identities/$personId.tsx

import { createFileRoute } from '@tanstack/react-router'
import { PersonDetailPage } from '@/components/identity/person-detail-page'

export const Route = createFileRoute('/_authed/identities/$personId')({
  component: PersonDetailPage,
})
