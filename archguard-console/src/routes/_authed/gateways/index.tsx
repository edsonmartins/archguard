import { createFileRoute } from '@tanstack/react-router'
import { GatewaysPage } from '@/components/gateways/gateways-page'

export const Route = createFileRoute('/_authed/gateways/')({
  component: GatewaysPage,
})
