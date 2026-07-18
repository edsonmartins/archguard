// src/lib/api/vault-client.ts
// Vault page status — backed by OpenBao (not AliasVault /api/vault).

import type { VaultStatus } from './types/vault'
import { getOpenBaoStatusFn } from '@/server/openbao-fn'

function offlineStatus(): VaultStatus {
  return {
    online: false,
    totalVaults: 0,
    activeAliases: 0,
    smtp: {
      online: false,
      mxConfigured: false,
      spfValid: false,
      dkimValid: false,
    },
  }
}

export const vaultApi = {
  /**
   * Map OpenBao health → VaultStatus for the Vault dashboard.
   * Tokens stay server-side via getOpenBaoStatusFn.
   */
  status: async (): Promise<VaultStatus & {
    sealed?: boolean
    initialized?: boolean
    cluster?: string
    addr?: string
    token_configured?: boolean
    token_kind?: string
    error?: string
  }> => {
    try {
      const s = await getOpenBaoStatusFn()
      if (!s.configured) return { ...offlineStatus(), addr: s.addr }

      const health = s.health as
        | {
            sealed?: boolean
            initialized?: boolean
            version?: string
            cluster_name?: string
          }
        | null
        | undefined
      const seal = s.seal as { sealed?: boolean } | null | undefined
      const sealed = health?.sealed ?? seal?.sealed
      const online =
        health != null &&
        sealed === false &&
        health.initialized !== false

      return {
        online,
        version: health?.version,
        totalVaults: online ? 1 : 0,
        totalPasswords: undefined,
        activeAliases: 0,
        smtp: {
          online: false,
          mxConfigured: false,
          spfValid: false,
          dkimValid: false,
        },
        sealed: sealed === true,
        initialized: health?.initialized === true,
        cluster: health?.cluster_name,
        addr: s.addr,
        token_configured: s.token_configured,
        token_kind: s.token_kind,
        error: (s as { error?: string }).error,
      }
    } catch {
      return offlineStatus()
    }
  },
}
