import { defineConfig } from '@playwright/test'

// Starts the real backend (in-memory store) and the Vite dev server, then
// drives the dashboard in a browser. Requires Go and `npm install`.
export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  use: { baseURL: 'http://127.0.0.1:5173', screenshot: 'only-on-failure' },
  webServer: [
    {
      command: 'go run ./apps/api/cmd/server',
      cwd: '../..',
      url: 'http://127.0.0.1:8080/health',
      env: { JWT_SECRET: 'e2e-secret-e2e-secret-e2e-secret-123', STORE: 'memory', APP_PORT: '8080' },
      reuseExistingServer: true,
      timeout: 120_000,
    },
    { command: 'npm run dev -- --host 127.0.0.1', url: 'http://127.0.0.1:5173', reuseExistingServer: true },
  ],
})
