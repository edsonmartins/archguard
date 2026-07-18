import { createFileRoute } from '@tanstack/react-router'
import { SiteOnboardingWizard } from '@/components/sites/site-onboarding-wizard'

export const Route = createFileRoute('/_authed/sites/wizard')({
  component: SiteOnboardingWizard,
})
