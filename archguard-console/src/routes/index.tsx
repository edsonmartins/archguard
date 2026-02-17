// src/routes/index.tsx

import { createFileRoute, redirect } from '@tanstack/react-router'

export const Route = createFileRoute('/')({
  beforeLoad: () => {
    // Redirect to dashboard; the _authed guard will redirect to /login if needed
    throw redirect({ to: '/dashboard' })
  },
})
