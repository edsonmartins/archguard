import { createFileRoute } from '@tanstack/react-router'
import { OraclePage } from '@/components/oracle/oracle-page'

export const Route = createFileRoute('/_authed/oracle/')({
  component: OraclePage,
})
