// src/routes/_authed/oauth2/index.tsx

import { createFileRoute } from '@tanstack/react-router'
import { OAuth2ListPage } from '@/components/oauth2/oauth2-list-page'

export const Route = createFileRoute('/_authed/oauth2/')({
  component: OAuth2ListPage,
})
