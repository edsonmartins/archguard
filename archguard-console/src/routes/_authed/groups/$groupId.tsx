// src/routes/_authed/groups/$groupId.tsx

import { createFileRoute } from '@tanstack/react-router'
import { GroupDetailPage } from '@/components/group/group-detail-page'

export const Route = createFileRoute('/_authed/groups/$groupId')({
  component: GroupDetailPage,
})
