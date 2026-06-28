// @vitest-environment jsdom
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ChatView } from './ChatView'
import { useChat } from './useChat'
import type { LogEntry } from '../api/types'
import type { Identity } from '../identity/types'

vi.mock('./useChat', () => ({
  useChat: vi.fn(),
}))

const identity: Identity = {
  senderId: 'alice',
  publicKey: new Uint8Array(),
  secretKey: new Uint8Array(),
}

function entry(senderId: string, text: string, sequence = 0): LogEntry {
  return {
    sequence,
    message: {
      schema_version: 1,
      message_id: 'id',
      chat_id: 'c1',
      sender_id: senderId,
      content: btoa(text),
      content_type: 'text/plain',
      encrypted: false,
      timestamp: '2026-01-01T00:00:00Z',
      public_key: 'cHVi',
      signature: 'c2ln',
    },
    prev_hash: '',
    entry_hash: 'hash',
  }
}

function mockUseChat(overrides: Partial<ReturnType<typeof useChat>> = {}) {
  vi.mocked(useChat).mockReturnValue({
    messages: [],
    caughtUp: true,
    error: null,
    loadMore: vi.fn(),
    send: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  })
}

beforeEach(() => {
  vi.mocked(useChat).mockReset()
})

describe('ChatView', () => {
  it('shows a loading status and a Load more button before catching up', () => {
    const loadMore = vi.fn()
    mockUseChat({ caughtUp: false, loadMore })
    render(<ChatView chatId="c1" identity={identity} />)

    expect(screen.getByText(/loading history/i)).toBeInTheDocument()
    screen.getByRole('button', { name: /load more/i }).click()
    expect(loadMore).toHaveBeenCalledTimes(1)
  })

  it('shows a live status and no Load more button once caught up', () => {
    mockUseChat({ caughtUp: true })
    render(<ChatView chatId="c1" identity={identity} />)

    expect(screen.getByText('live')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /load more/i })).not.toBeInTheDocument()
  })

  it('renders an error message when present', () => {
    mockUseChat({ error: 'boom' })
    render(<ChatView chatId="c1" identity={identity} />)

    expect(screen.getByText(/boom/)).toBeInTheDocument()
  })

  it('renders messages with the decoded text and marks the viewer’s own as "mine"', () => {
    mockUseChat({
      messages: [entry('alice', 'hi there', 0), entry('bob', 'hello', 1)],
    })
    render(<ChatView chatId="c1" identity={identity} />)

    expect(screen.getByText('hi there')).toBeInTheDocument()
    expect(screen.getByText('hello')).toBeInTheDocument()
    expect(screen.getByText('hi there').closest('li')).toHaveClass('mine')
    expect(screen.getByText('hello').closest('li')).not.toHaveClass('mine')
  })

  it('shows a placeholder instead of throwing on unreadable content', () => {
    const bad = entry('alice', 'irrelevant', 0)
    bad.message.content = 'not-valid-base64!!'
    mockUseChat({ messages: [bad] })

    render(<ChatView chatId="c1" identity={identity} />)
    expect(screen.getByText('(unreadable)')).toBeInTheDocument()
  })

  it('sends the trimmed draft and clears the input on submit', async () => {
    const send = vi.fn().mockResolvedValue(undefined)
    mockUseChat({ send })
    render(<ChatView chatId="c1" identity={identity} />)

    const input = screen.getByPlaceholderText('Type a message')
    await userEvent.type(input, '  hello world  ')
    await userEvent.click(screen.getByRole('button', { name: /send/i }))

    expect(send).toHaveBeenCalledWith('hello world')
    expect(input).toHaveValue('')
  })

  it('disables Send until there is a non-blank draft', async () => {
    mockUseChat()
    render(<ChatView chatId="c1" identity={identity} />)

    const sendButton = screen.getByRole('button', { name: /send/i })
    expect(sendButton).toBeDisabled()

    await userEvent.type(screen.getByPlaceholderText('Type a message'), 'hi')
    expect(sendButton).toBeEnabled()
  })
})
