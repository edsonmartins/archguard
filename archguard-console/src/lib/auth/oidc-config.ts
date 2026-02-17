// src/lib/auth/oidc-config.ts

import { UserManager, WebStorageStateStore } from 'oidc-client-ts'

const ARCHGUARD_ID_URL =
  typeof window !== 'undefined'
    ? (import.meta.env.VITE_ARCHGUARD_ID_URL as string)
    : ''

export function createUserManager() {
  return new UserManager({
    authority: `${ARCHGUARD_ID_URL}/oauth2/openid/archguard-console`,
    client_id: 'archguard-console',
    redirect_uri: `${window.location.origin}/callback`,
    post_logout_redirect_uri: window.location.origin,
    response_type: 'code',
    scope: 'openid profile email groups',
    automaticSilentRenew: false, // We handle refresh server-side
    userStore: new WebStorageStateStore({ store: sessionStorage }),
  })
}

let _userManager: UserManager | null = null

export function getUserManager(): UserManager {
  if (!_userManager) {
    _userManager = createUserManager()
  }
  return _userManager
}
