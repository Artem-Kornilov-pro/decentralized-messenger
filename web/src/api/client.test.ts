import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiRequestError, fetchHistory, sendText } from './client'
import type { SignedMessageJSON } from '../crypto/signedMessage'

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('api client', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('POSTs a signed message to /chats/{chatID}/messages', async () => {
    const entry = { sequence: 0, message: {}, prev_hash: '', entry_hash: 'abc' }
    vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(entry, 201))

    const msg = { chat_id: 'demo' } as unknown as SignedMessageJSON
    const got = await sendText('demo', msg)

    expect(got.entry_hash).toBe('abc')
    const [url, init] = vi.mocked(fetch).mock.calls[0]
    expect(url).toBe('/chats/demo/messages')
    expect(init?.method).toBe('POST')
    expect(JSON.parse(init?.body as string)).toEqual(msg)
  })

  it('fetches a history page with from/limit query params', async () => {
    const page = { messages: [], next_from: null }
    vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(page))

    await fetchHistory('demo', 5, 10)

    const [url] = vi.mocked(fetch).mock.calls[0]
    expect(url).toBe('/chats/demo/messages?from=5&limit=10')
  })

  it('throws ApiRequestError with the server error message on failure', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(jsonResponse({ error: 'bad signature' }, 422))

    await expect(sendText('demo', {} as SignedMessageJSON)).rejects.toMatchObject(
      new ApiRequestError(422, 'bad signature'),
    )
  })
})
