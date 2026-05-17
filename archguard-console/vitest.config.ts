import { defineConfig } from 'vitest/config'
import viteTsConfigPaths from 'vite-tsconfig-paths'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  plugins: [viteTsConfigPaths({ projects: ['./tsconfig.json'] })],
  test: {
    environment: 'jsdom',
    setupFiles: ['./tests/setup.ts'],
    include: ['tests/unit/**/*.test.{ts,tsx}'],
    exclude: ['tests/e2e/**', 'node_modules/**'],
    globals: true,
    coverage: {
      provider: 'v8',
      include: ['src/server/**', 'src/lib/auth/**', 'src/lib/api/normalizers.ts'],
      exclude: ['**/*.d.ts', 'src/server/activity-log.ts'],
      reporter: ['text', 'html', 'lcov'],
    },
  },
})
