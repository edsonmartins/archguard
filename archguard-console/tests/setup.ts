// tests/setup.ts
// Global test setup — runs before each test file.

import '@testing-library/jest-dom/vitest'

// Stable session secret used by session.ts encrypt/decrypt tests.
// Real deployments must override SESSION_SECRET via env.
process.env.SESSION_SECRET =
  '0000000000000000000000000000000000000000000000000000000000000000'
