// src/routes/_authed/groups/create.tsx

import { createFileRoute } from '@tanstack/react-router'
import { GroupCreatePage } from '@/components/group/group-create-page'

export const Route = createFileRoute('/_authed/groups/create')({
  component: GroupCreatePage,
})
