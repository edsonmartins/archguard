import { createFileRoute } from '@tanstack/react-router'
import { SiteDetailPage } from '@/components/sites/site-detail-page'

export const Route = createFileRoute('/_authed/sites/$slug')({
  component: SiteDetailRoute,
})

function SiteDetailRoute() {
  const { slug } = Route.useParams()
  return <SiteDetailPage slug={slug} />
}
