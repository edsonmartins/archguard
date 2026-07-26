import { createFileRoute } from '@tanstack/react-router'
import { OrgAccountListPage } from '@/components/org-accounts/org-account-list-page'

export const Route = createFileRoute('/_authed/org-accounts/')({
  component: OrgAccountListPage,
})
