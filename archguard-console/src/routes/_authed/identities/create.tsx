// src/routes/_authed/identities/create.tsx

import { createFileRoute } from '@tanstack/react-router'
import { PersonFormWizard } from '@/components/identity/person-form-wizard'

export const Route = createFileRoute('/_authed/identities/create')({
  component: PersonFormWizard,
})
