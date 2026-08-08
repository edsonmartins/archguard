// tests/e2e/fixtures/test-data.ts
//
// Test users provisioned by scripts/setup-archguard.sh.
// Keep in sync with that script.

export const TEST_USERS = {
  admin: {
    username: 'testadmin',
    password: 'ArchGuard2026TestAdmin',
    expectedGroups: ['archguard_admins'],
  },
  user: {
    username: 'testuser',
    password: 'ArchGuard2026TestUser',
    expectedGroups: ['archguard_users'],
  },
} as const

export const archguard_URL =
  process.env.E2E_archguard_URL ?? 'https://localhost:8443'
