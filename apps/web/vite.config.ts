import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// In development the API runs on :8080; the proxy keeps requests same-origin,
// so the browser never needs CORS and tokens are never sent cross-origin.
export default defineConfig({
  plugins: [react()],
  server: { port: 5173, proxy: { '/api': 'http://127.0.0.1:8080' } },
  test: { environment: 'jsdom', include: ['src/**/*.test.{ts,tsx}'] },
})
