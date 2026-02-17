// src/routes/_authed/identities/import.tsx

import { createFileRoute } from '@tanstack/react-router'
import { CsvImportWizard } from '@/components/identity/csv-import-wizard'

export const Route = createFileRoute('/_authed/identities/import')({
  component: CsvImportWizard,
})
