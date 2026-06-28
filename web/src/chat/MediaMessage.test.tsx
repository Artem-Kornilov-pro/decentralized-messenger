// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MediaMessage } from './MediaMessage'
import { decrypt } from '../crypto/aesgcm'
import type { LogEntry } from '../api/types'

vi.mock('../crypto/aesgcm', () => ({
  decrypt: vi.fn(),
}))

function entry(contentType: string, filename = ''): LogEntry {
  return {
    sequence: 0,
    message: {
      schema_version: 1,
      message_id: 'id',
      chat_id: 'c1',
      sender_id: 'alice',
      content: btoa('ciphertext'),
      content_type: contentType,
      filename,
      encrypted: true,
      timestamp: '2026-01-01T00:00:00Z',
      public_key: 'cHVi',
      signature: 'c2ln',
    },
    prev_hash: '',
    entry_hash: 'hash',
  }
}

const contentKey = new Uint8Array(32).fill(3)

beforeEach(() => {
  URL.createObjectURL = vi.fn(() => 'blob:fake-url')
  URL.revokeObjectURL = vi.fn()
})

afterEach(() => {
  cleanup()
})

describe('MediaMessage', () => {
  it('shows a placeholder and never decrypts when there is no content key', () => {
    render(<MediaMessage entry={entry('image/jpeg')} contentKey={null} />)

    expect(screen.getByText(/set the content key to view/)).toBeInTheDocument()
    expect(decrypt).not.toHaveBeenCalled()
  })

  it('renders an <img> for image content after decrypting', async () => {
    vi.mocked(decrypt).mockResolvedValueOnce(new TextEncoder().encode('plaintext'))
    render(<MediaMessage entry={entry('image/jpeg', 'cat.jpg')} contentKey={contentKey} />)

    const img = await screen.findByRole('img')
    expect(img).toHaveAttribute('src', 'blob:fake-url')
    expect(img).toHaveAttribute('alt', 'cat.jpg')
    expect(decrypt).toHaveBeenCalledWith(contentKey, expect.any(Uint8Array))
  })

  it('renders a <video> for video content after decrypting', async () => {
    vi.mocked(decrypt).mockResolvedValueOnce(new TextEncoder().encode('plaintext'))
    const { container } = render(<MediaMessage entry={entry('video/mp4')} contentKey={contentKey} />)

    await waitFor(() => expect(container.querySelector('video')).not.toBeNull())
    expect(container.querySelector('video')).toHaveAttribute('src', 'blob:fake-url')
  })

  it('shows an error placeholder when decryption fails', async () => {
    vi.mocked(decrypt).mockRejectedValueOnce(new Error('bad key'))
    render(<MediaMessage entry={entry('image/jpeg')} contentKey={contentKey} />)

    expect(await screen.findByText(/couldn't decrypt/)).toBeInTheDocument()
  })

  it('revokes the object URL on unmount', async () => {
    vi.mocked(decrypt).mockResolvedValueOnce(new TextEncoder().encode('plaintext'))
    const { container, unmount } = render(
      <MediaMessage entry={entry('image/jpeg')} contentKey={contentKey} />,
    )

    await waitFor(() => expect(container.querySelector('img')).not.toBeNull())
    unmount()

    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:fake-url')
  })
})
