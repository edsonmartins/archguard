// src/lib/utils/constants.ts

export const BUILTIN_GROUPS = new Set([
  'idm_admins',
  'idm_people_admins',
  'idm_oauth2_admins',
  'idm_service_desk',
  'idm_people_on_boarding',
  'idm_all_accounts',
  'idm_all_persons',
  'system_admins',
  'idm_hp_account_manage',
])

export const ADMIN_GROUPS = [
  'archguard_admins',
  'idm_admins',
  'idm_people_admins',
  'idm_oauth2_admins',
]

export const DEFAULT_SCOPES = ['openid', 'email', 'profile', 'groups']

export const DEFAULT_PAGE_SIZE = 25

export const MAX_CSV_IMPORT = 500

export const OIDC_CLIENT_ID = 'archguard-console'

export const PROTECTED_OAUTH2_CLIENTS = new Set(['archguard-console'])
