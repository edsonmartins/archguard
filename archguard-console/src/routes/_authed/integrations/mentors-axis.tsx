import { createFileRoute } from '@tanstack/react-router'
import { MentorsAxisPage } from '@/components/integrations/mentors-axis-page'

export const Route = createFileRoute('/_authed/integrations/mentors-axis')({
  component: MentorsAxisPage,
})
