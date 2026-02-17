// src/components/shared/time-ago.tsx

import { timeAgo } from '@/lib/utils/formatters'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatDate } from '@/lib/utils/formatters'

interface TimeAgoProps {
  date: Date | string
}

export function TimeAgo({ date }: TimeAgoProps) {
  const d = typeof date === 'string' ? new Date(date) : date
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="cursor-default text-muted-foreground">
          {timeAgo(d)}
        </span>
      </TooltipTrigger>
      <TooltipContent>{formatDate(d)}</TooltipContent>
    </Tooltip>
  )
}
