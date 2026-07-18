import { createFileRoute } from '@tanstack/react-router'
import { SiteFormPage } from '@/components/sites/site-form-page'

export const Route = createFileRoute('/_authed/sites/create')({
  component: () => <SiteFormPage mode="create" />,
})
