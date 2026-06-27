import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// The Go API node (cmd/messenger) listens on :8080 by default. Proxying it
// here makes the dev server same-origin from the browser's point of view, so
// no CORS configuration is needed on the backend for local development.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/chats': { target: 'http://localhost:8080', ws: true },
      '/keys': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
    },
  },
  // Default environment is Node (vitest's default) so the WebCrypto-backed
  // ed25519 tests use Node's native crypto.subtle. Tests that need
  // localStorage/DOM opt into jsdom per-file via a `@vitest-environment
  // jsdom` pragma — jsdom's crypto.subtle runs typed arrays in a different
  // realm, which breaks @noble/ed25519's instanceof checks if applied
  // globally.
  test: {
    setupFiles: ['./src/setupTests.ts'],
  },
})
