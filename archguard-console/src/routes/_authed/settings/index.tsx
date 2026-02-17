// src/routes/_authed/settings/index.tsx

import { createFileRoute } from '@tanstack/react-router'
import { SettingsPage } from '@/components/settings/settings-page'

export const Route = createFileRoute('/_authed/settings/')({
  component: SettingsPage,
})
