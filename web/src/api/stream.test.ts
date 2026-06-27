// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { openStream } from './stream'

class FakeWebSocket {
  static instances: FakeWebSocket[] = []
  url: string
  onmessage: ((e: MessageEvent) => void) | null = null
  close = vi.fn()

  constructor(url: string) {
    this.url = url
    FakeWebSocket.instances.push(this)
  }
}

beforeEach(() => {
  FakeWebSocket.instances = []
  vi.stubGlobal('WebSocket', FakeWebSocket)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('openStream', () => {
  it('connects to ws://<host>/chats/{chatID}/ws on http pages', () => {
    openStream('demo', vi.fn())
    expect(FakeWebSocket.instances).toHaveLength(1)
    expect(FakeWebSocket.instances[0].url).toBe(`ws://${location.host}/chats/demo/ws`)
  })

  it('URL-encodes the chat ID', () => {
    openStream('chat with spaces', vi.fn())
    expect(FakeWebSocket.instances[0].url).toBe(
      `ws://${location.host}/chats/chat%20with%20spaces/ws`,
    )
  })

  it('uses wss:// on https pages', () => {
    const original = window.location
    Object.defineProperty(window, 'location', {
      value: { ...original, protocol: 'https:', host: original.host },
      writable: true,
    })

    openStream('demo', vi.fn())
    expect(FakeWebSocket.instances[0].url).toBe(`wss://${original.host}/chats/demo/ws`)

    Object.defineProperty(window, 'location', { value: original, writable: true })
  })

  it('decodes incoming frames and forwards them to onEvent', () => {
    const onEvent = vi.fn()
    openStream('demo', onEvent)
    const evt = { kind: 'entry_appended', chat_id: 'demo', sequence: 1, entry_hash: 'h' }

    FakeWebSocket.instances[0].onmessage?.({ data: JSON.stringify(evt) } as MessageEvent)

    expect(onEvent).toHaveBeenCalledWith(evt)
  })

  it('silently ignores malformed frames instead of throwing', () => {
    const onEvent = vi.fn()
    openStream('demo', onEvent)

    expect(() =>
      FakeWebSocket.instances[0].onmessage?.({ data: 'not json' } as MessageEvent),
    ).not.toThrow()
    expect(onEvent).not.toHaveBeenCalled()
  })

  it('returns a function that closes the socket', () => {
    const close = openStream('demo', vi.fn())
    close()
    expect(FakeWebSocket.instances[0].close).toHaveBeenCalledTimes(1)
  })
})
