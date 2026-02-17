// src/routes/_authed/identities/index.tsx

import { createFileRoute } from '@tanstack/react-router'
import { PersonListPage } from '@/components/identity/person-list-page'

export const Route = createFileRoute('/_authed/identities/')({
  component: PersonListPage,
})
