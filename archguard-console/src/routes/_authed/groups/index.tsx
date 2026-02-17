// src/routes/_authed/groups/index.tsx

import { createFileRoute } from '@tanstack/react-router'
import { GroupListPage } from '@/components/group/group-list-page'

export const Route = createFileRoute('/_authed/groups/')({
  component: GroupListPage,
})
