// src/components/shared/empty-state.tsx

import { Link } from '@tanstack/react-router'
import { InboxIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface EmptyStateProps {
  icon?: React.ComponentType<{ className?: string }>
  title: string
  description: string
  action?: {
    label: string
    to: string
  }
}

export function EmptyState({
  icon: Icon = InboxIcon,
  title,
  description,
  action,
}: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <Icon className="mb-4 h-12 w-12 text-muted-foreground/50" />
      <h3 className="mb-1 text-lg font-medium">{title}</h3>
      <p className="mb-4 max-w-sm text-sm text-muted-foreground">
        {description}
      </p>
      {action && (
        <Button asChild>
          <Link to={action.to as string}>{action.label}</Link>
        </Button>
      )}
    </div>
  )
}
