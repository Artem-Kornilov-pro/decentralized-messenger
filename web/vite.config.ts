import { defineConfig } from 'vite'
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
})
