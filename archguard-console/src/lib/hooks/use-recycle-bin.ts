// src/lib/hooks/use-recycle-bin.ts

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { recycleBinApi } from '@/lib/api/kanidm-client'
import { mapKanidmError } from '@/lib/utils/error-mapper'
import type { RecycleBinEntry } from '@/lib/api/types/kanidm'

export function useRecycleBin() {
  return useQuery<RecycleBinEntry[]>({
    queryKey: ['recycleBin'],
    queryFn: () => recycleBinApi.list(),
    staleTime: 30_000,
  })
}

export function useReviveEntry() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => recycleBinApi.revive(id),
    onSuccess: () => {
      toast.success('Item restaurado com sucesso')
      queryClient.invalidateQueries({ queryKey: ['recycleBin'] })
      // Also invalidate all entity lists since the revived item may appear
      queryClient.invalidateQueries({ queryKey: ['persons'] })
      queryClient.invalidateQueries({ queryKey: ['groups'] })
      queryClient.invalidateQueries({ queryKey: ['serviceAccounts'] })
      queryClient.invalidateQueries({ queryKey: ['oauth2'] })
    },
    onError: (err) => {
      toast.error(mapKanidmError(err))
    },
  })
}
