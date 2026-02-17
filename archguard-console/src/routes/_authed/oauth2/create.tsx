// src/routes/_authed/oauth2/create.tsx

import { createFileRoute } from '@tanstack/react-router'
import { OAuth2CreateWizard } from '@/components/oauth2/oauth2-create-wizard'

export const Route = createFileRoute('/_authed/oauth2/create')({
  component: OAuth2CreateWizard,
})
