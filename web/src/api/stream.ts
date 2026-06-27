import type { StreamEvent } from './types'

/** Opens a WebSocket to /chats/{chatID}/ws and calls onEvent for every
 * decoded event. Returns a function that closes the socket. Malformed
 * frames are ignored (the stream is a best-effort notification channel —
 * callers always re-derive truth from the REST history endpoint). */
export function openStream(chatId: string, onEvent: (evt: StreamEvent) => void): () => void {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const ws = new WebSocket(`${proto}//${location.host}/chats/${encodeURIComponent(chatId)}/ws`)

  ws.onmessage = (e) => {
    try {
      onEvent(JSON.parse(e.data))
    } catch {
      // ignore malformed frames
    }
  }

  return () => ws.close()
}
