import { createFileRoute } from '@tanstack/react-router'
import { SiteFormPage } from '@/components/sites/site-form-page'

export const Route = createFileRoute('/_authed/sites/$slug/edit')({
  component: SiteEditRoute,
})

function SiteEditRoute() {
  const { slug } = Route.useParams()
  return <SiteFormPage mode="edit" slug={slug} />
}
