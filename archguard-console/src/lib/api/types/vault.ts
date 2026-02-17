// src/lib/api/types/vault.ts

export interface VaultStatus {
  online: boolean
  version?: string
  totalVaults: number
  totalPasswords?: number
  activeAliases: number
  smtp: SmtpStatus
}

export interface SmtpStatus {
  online: boolean
  domain?: string
  mxConfigured: boolean
  spfValid: boolean
  dkimValid: boolean
}
