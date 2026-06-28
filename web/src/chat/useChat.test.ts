// @vitest-environment jsdom
import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useChat } from './useChat'
import { fetchHistory, sendPhoto, sendText, sendVideo } from '../api/client'
import { openStream } from '../api/stream'
import { sign } from '../crypto/ed25519'
import { encrypt } from '../crypto/aesgcm'
import type { Identity } from '../identity/types'
import type { LogEntry } from '../api/types'
import type { StreamEvent } from '../api/types'

vi.mock('../api/client', () => ({
  fetchHistory: vi.fn(),
  sendText: vi.fn(),
  sendPhoto: vi.fn(),
  sendVideo: vi.fn(),
}))
vi.mock('../api/stream', () => ({
  openStream: vi.fn(),
}))
// useChat's send()/sendAttachment() sign/encrypt via the real ed25519/aesgcm
// modules, which use crypto.subtle internally. jsdom's crypto.subtle runs
// typed arrays in a different realm than @noble/ed25519 expects (see
// crypto/ed25519.ts's test file for the same issue), so both are mocked
// here — correctness is already covered by ed25519.test.ts, aesgcm.test.ts,
// and signedMessage.test.ts; this file only cares about useChat's own
// orchestration. Re-applied in beforeEach since resetAllMocks (afterEach)
// would otherwise wipe these implementations too.
vi.mock('../crypto/ed25519', () => ({
  sign: vi.fn(),
}))
vi.mock('../crypto/aesgcm', () => ({
  encrypt: vi.fn(),
}))

const identity: Identity = {
  senderId: 'alice',
  publicKey: new Uint8Array(32).fill(1),
  secretKey: new Uint8Array(32).fill(2),
}

function entry(sequence: number): LogEntry {
  return {
    sequence,
    message: {
      schema_version: 1,
      message_id: `id-${sequence}`,
      chat_id: 'c1',
      sender_id: 'alice',
      content: btoa(`msg-${sequence}`),
      content_type: 'text/plain',
      encrypted: false,
      timestamp: '2026-01-01T00:00:00Z',
      public_key: 'cHVi',
      signature: 'c2ln',
    },
    prev_hash: '',
    entry_hash: `hash-${sequence}`,
  }
}

let streamHandlers: Map<string, (evt: StreamEvent) => void>
let closeStream: () => void

beforeEach(() => {
  streamHandlers = new Map()
  closeStream = vi.fn<() => void>()
  vi.mocked(sign).mockResolvedValue(new Uint8Array([9, 9, 9]))
  vi.mocked(encrypt).mockImplementation((_key, plaintext) =>
    Promise.resolve(new Uint8Array([...new Uint8Array(12), ...plaintext])),
  )

  vi.mocked(openStream).mockImplementation((chatId, onEvent) => {
    streamHandlers.set(chatId, onEvent)
    return closeStream
  })
})

afterEach(() => {
  vi.resetAllMocks()
})

describe('useChat', () => {
  it('loads history and marks caught up when next_from is null', async () => {
    vi.mocked(fetchHistory).mockResolvedValueOnce({
      messages: [entry(0), entry(1)],
      next_from: null,
    })

    const { result } = renderHook(() => useChat('c1', identity))

    await waitFor(() => expect(result.current.caughtUp).toBe(true))
    expect(result.current.messages.map((e) => e.sequence)).toEqual([0, 1])
    expect(fetchHistory).toHaveBeenCalledWith('c1', 0, 50)
    // Only opens the live stream once caught up.
    expect(openStream).toHaveBeenCalledTimes(1)
  })

  it('pages forward through history before catching up', async () => {
    vi.mocked(fetchHistory)
      .mockResolvedValueOnce({ messages: [entry(0)], next_from: 1 })
      .mockResolvedValueOnce({ messages: [entry(1)], next_from: null })

    const { result } = renderHook(() => useChat('c1', identity))

    await waitFor(() => expect(result.current.messages).toHaveLength(1))
    expect(result.current.caughtUp).toBe(false)
    expect(openStream).not.toHaveBeenCalled()

    await act(() => result.current.loadMore())

    expect(fetchHistory).toHaveBeenNthCalledWith(2, 'c1', 1, 50)
    expect(result.current.caughtUp).toBe(true)
    expect(result.current.messages.map((e) => e.sequence)).toEqual([0, 1])
  })

  it('fetches and appends new entries on a live entry_appended event', async () => {
    vi.mocked(fetchHistory).mockResolvedValueOnce({ messages: [entry(0)], next_from: null })
    const { result } = renderHook(() => useChat('c1', identity))
    await waitFor(() => expect(result.current.caughtUp).toBe(true))

    vi.mocked(fetchHistory).mockResolvedValueOnce({ messages: [entry(1)], next_from: null })
    await act(async () => {
      streamHandlers.get('c1')?.({ kind: 'entry_appended', chat_id: 'c1', sequence: 1, entry_hash: 'h' })
      await Promise.resolve()
    })

    await waitFor(() => expect(result.current.messages.map((e) => e.sequence)).toEqual([0, 1]))
    expect(fetchHistory).toHaveBeenLastCalledWith('c1', 1, 50)
  })

  it('ignores stream events for other chats or other kinds', async () => {
    vi.mocked(fetchHistory).mockResolvedValueOnce({ messages: [entry(0)], next_from: null })
    const { result } = renderHook(() => useChat('c1', identity))
    await waitFor(() => expect(result.current.caughtUp).toBe(true))

    const callsBefore = vi.mocked(fetchHistory).mock.calls.length
    await act(async () => {
      streamHandlers.get('c1')?.({ kind: 'entry_appended', chat_id: 'other-chat', sequence: 1, entry_hash: 'h' })
      streamHandlers.get('c1')?.({ kind: 'snapshot_created', chat_id: 'c1', sequence: 1, entry_hash: 'h' })
      await Promise.resolve()
    })

    expect(fetchHistory).toHaveBeenCalledTimes(callsBefore)
  })

  it('dedupes entries that arrive via both the optimistic send and a live refetch', async () => {
    vi.mocked(fetchHistory).mockResolvedValueOnce({ messages: [], next_from: null })
    const { result } = renderHook(() => useChat('c1', identity))
    await waitFor(() => expect(result.current.caughtUp).toBe(true))

    vi.mocked(sendText).mockResolvedValueOnce(entry(0))
    await act(() => result.current.send('hello'))
    expect(result.current.messages).toHaveLength(1)

    // A WS-triggered refetch for the same range redelivers sequence 0.
    vi.mocked(fetchHistory).mockResolvedValueOnce({ messages: [entry(0)], next_from: null })
    await act(async () => {
      streamHandlers.get('c1')?.({ kind: 'entry_appended', chat_id: 'c1', sequence: 0, entry_hash: 'h' })
      await Promise.resolve()
    })

    expect(result.current.messages).toHaveLength(1)
  })

  it('send() signs and submits the message, then appends the stored entry', async () => {
    vi.mocked(fetchHistory).mockResolvedValueOnce({ messages: [], next_from: null })
    const { result } = renderHook(() => useChat('c1', identity))
    await waitFor(() => expect(result.current.caughtUp).toBe(true))

    vi.mocked(sendText).mockResolvedValueOnce(entry(0))
    await act(() => result.current.send('hello'))

    expect(sendText).toHaveBeenCalledTimes(1)
    const [chatId, wire] = vi.mocked(sendText).mock.calls[0]
    expect(chatId).toBe('c1')
    expect(wire.chat_id).toBe('c1')
    expect(wire.sender_id).toBe('alice')
    expect(atob(wire.content)).toBe('hello')
    expect(wire.signature).toBeTruthy()
    expect(result.current.messages).toHaveLength(1)
  })

  it('surfaces an error from a failed history fetch', async () => {
    vi.mocked(fetchHistory).mockRejectedValueOnce(new Error('boom'))
    const { result } = renderHook(() => useChat('c1', identity))

    await waitFor(() => expect(result.current.error).toBe('boom'))
  })

  it('surfaces an error from a failed send', async () => {
    vi.mocked(fetchHistory).mockResolvedValueOnce({ messages: [], next_from: null })
    const { result } = renderHook(() => useChat('c1', identity))
    await waitFor(() => expect(result.current.caughtUp).toBe(true))

    vi.mocked(sendText).mockRejectedValueOnce(new Error('rejected'))
    await act(() => result.current.send('hello'))

    expect(result.current.error).toBe('rejected')
  })

  it('resets state and closes the previous stream when chatId changes', async () => {
    vi.mocked(fetchHistory).mockResolvedValueOnce({ messages: [entry(0)], next_from: null })
    const { result, rerender } = renderHook(({ chatId }) => useChat(chatId, identity), {
      initialProps: { chatId: 'c1' },
    })
    await waitFor(() => expect(result.current.caughtUp).toBe(true))

    vi.mocked(fetchHistory).mockResolvedValueOnce({ messages: [entry(0)], next_from: null })
    rerender({ chatId: 'c2' })

    await waitFor(() => expect(fetchHistory).toHaveBeenLastCalledWith('c2', 0, 50))
    // The old chat's stream is torn down at some point during the reset;
    // exact effect-cleanup invocation count isn't the contract worth pinning
    // down — what matters is that it happens, and that c2 gets a fresh one.
    expect(closeStream).toHaveBeenCalled()
    await waitFor(() => expect(result.current.messages.map((e) => e.sequence)).toEqual([0]))
    await waitFor(() => expect(openStream).toHaveBeenLastCalledWith('c2', expect.any(Function)))
  })

  describe('sendAttachment', () => {
    const contentKey = new Uint8Array(32).fill(4)

    function makeFile(name: string, type: string, size: number): File {
      return new File([new Uint8Array(size)], name, { type })
    }

    async function caughtUpHook() {
      vi.mocked(fetchHistory).mockResolvedValueOnce({ messages: [], next_from: null })
      const { result } = renderHook(() => useChat('c1', identity))
      await waitFor(() => expect(result.current.caughtUp).toBe(true))
      return result
    }

    it('encrypts and posts an image to /photos, appending the stored entry', async () => {
      const result = await caughtUpHook()
      const file = makeFile('cat.jpg', 'image/jpeg', 1024)
      vi.mocked(sendPhoto).mockResolvedValueOnce(entry(0))

      await act(() => result.current.sendAttachment(file, contentKey))

      expect(encrypt).toHaveBeenCalledWith(contentKey, expect.any(Uint8Array))
      expect(sendPhoto).toHaveBeenCalledTimes(1)
      const [chatId, wire] = vi.mocked(sendPhoto).mock.calls[0]
      expect(chatId).toBe('c1')
      expect(wire.content_type).toBe('image/jpeg')
      expect(wire.filename).toBe('cat.jpg')
      expect(wire.encrypted).toBe(true)
      expect(sendVideo).not.toHaveBeenCalled()
      expect(result.current.messages).toHaveLength(1)
    })

    it('posts a video to /videos instead of /photos', async () => {
      const result = await caughtUpHook()
      const file = makeFile('clip.mp4', 'video/mp4', 1024)
      vi.mocked(sendVideo).mockResolvedValueOnce(entry(0))

      await act(() => result.current.sendAttachment(file, contentKey))

      expect(sendVideo).toHaveBeenCalledTimes(1)
      expect(sendPhoto).not.toHaveBeenCalled()
    })

    it('rejects an oversize photo client-side without calling the API', async () => {
      const result = await caughtUpHook()
      const file = makeFile('huge.jpg', 'image/jpeg', 10 * 1024 * 1024 + 1)

      await act(() => result.current.sendAttachment(file, contentKey))

      expect(result.current.error).toMatch(/exceeds/)
      expect(encrypt).not.toHaveBeenCalled()
      expect(sendPhoto).not.toHaveBeenCalled()
    })

    it('rejects an oversize video client-side without calling the API', async () => {
      const result = await caughtUpHook()
      const file = makeFile('huge.mp4', 'video/mp4', 50 * 1024 * 1024 + 1)

      await act(() => result.current.sendAttachment(file, contentKey))

      expect(result.current.error).toMatch(/exceeds/)
      expect(sendVideo).not.toHaveBeenCalled()
    })

    it('rejects an unsupported file type', async () => {
      const result = await caughtUpHook()
      const file = makeFile('doc.pdf', 'application/pdf', 1024)

      await act(() => result.current.sendAttachment(file, contentKey))

      expect(result.current.error).toMatch(/unsupported file type/)
      expect(encrypt).not.toHaveBeenCalled()
      expect(sendPhoto).not.toHaveBeenCalled()
      expect(sendVideo).not.toHaveBeenCalled()
    })

    it('surfaces an error if encryption fails', async () => {
      const result = await caughtUpHook()
      vi.mocked(encrypt).mockRejectedValueOnce(new Error('encrypt failed'))
      const file = makeFile('cat.jpg', 'image/jpeg', 1024)

      await act(() => result.current.sendAttachment(file, contentKey))

      expect(result.current.error).toBe('encrypt failed')
      expect(sendPhoto).not.toHaveBeenCalled()
    })
  })
})
