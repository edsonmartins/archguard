import { createFileRoute } from '@tanstack/react-router'
import { SiteListPage } from '@/components/sites/site-list-page'

export const Route = createFileRoute('/_authed/sites/')({
  component: SiteListPage,
})
