// src/routes/_authed/recycle-bin.tsx

import { createFileRoute } from '@tanstack/react-router'
import { RecycleBinPage } from '@/components/recycle-bin/recycle-bin-page'

export const Route = createFileRoute('/_authed/recycle-bin')({
  component: RecycleBinPage,
})
