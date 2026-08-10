import { createFileRoute } from '@tanstack/react-router'
import { ConnectorRolloutsPage } from '@/components/sites/connector-rollouts-page'

export const Route = createFileRoute('/_authed/sites/rollouts')({
  component: ConnectorRolloutsPage,
})
