import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// Not using vitest's `globals: true`, so testing-library's own
// auto-cleanup-on-afterEach detection doesn't fire — unmount explicitly,
// or component tests leak DOM nodes across tests within the same file.
afterEach(() => {
  cleanup()
})
