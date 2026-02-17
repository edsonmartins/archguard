// src/lib/api/vault-client.ts

import type { VaultStatus } from './types/vault'

const VAULT_URL =
  typeof window !== 'undefined'
    ? '/api/vault'
    : process.env.ARCHGUARD_VAULT_URL || 'https://localhost'

export const vaultApi = {
  status: async (): Promise<VaultStatus> => {
    try {
      const response = await fetch(`${VAULT_URL}/api/v1/status`)
      if (!response.ok) {
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
      return response.json()
    } catch {
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
  },
}
