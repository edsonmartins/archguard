// src/routes/_authed/oauth2/$clientId.tsx

import { createFileRoute } from '@tanstack/react-router'
import { OAuth2DetailPage } from '@/components/oauth2/oauth2-detail-page'

export const Route = createFileRoute('/_authed/oauth2/$clientId')({
  component: OAuth2DetailPage,
})
