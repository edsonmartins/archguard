// src/routes/_authed/audit.tsx

import { createFileRoute } from '@tanstack/react-router'
import { AuditPage } from '@/components/audit/audit-page'

export const Route = createFileRoute('/_authed/audit')({
  component: AuditPage,
})
