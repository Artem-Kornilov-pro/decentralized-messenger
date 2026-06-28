import type { SignedMessageJSON } from '../crypto/signedMessage'
import type { ApiError, HistoryResponse, LogEntry } from './types'

export class ApiRequestError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function parseOrThrow<T>(resp: Response): Promise<T> {
  if (!resp.ok) {
    let message = resp.statusText
    try {
      const body: ApiError = await resp.json()
      if (body.error) message = body.error
    } catch {
      // body wasn't JSON; fall back to statusText
    }
    throw new ApiRequestError(resp.status, message)
  }
  return resp.json()
}

async function postSignedMessage(
  chatId: string,
  resource: 'messages' | 'photos' | 'videos',
  msg: SignedMessageJSON,
): Promise<LogEntry> {
  const resp = await fetch(`/chats/${encodeURIComponent(chatId)}/${resource}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(msg),
  })
  return parseOrThrow<LogEntry>(resp)
}

/** POSTs a pre-signed message to /chats/{chatID}/messages. */
export function sendText(chatId: string, msg: SignedMessageJSON): Promise<LogEntry> {
  return postSignedMessage(chatId, 'messages', msg)
}

/** POSTs a pre-signed, encrypted photo to /chats/{chatID}/photos. */
export function sendPhoto(chatId: string, msg: SignedMessageJSON): Promise<LogEntry> {
  return postSignedMessage(chatId, 'photos', msg)
}

/** POSTs a pre-signed, encrypted video to /chats/{chatID}/videos. */
export function sendVideo(chatId: string, msg: SignedMessageJSON): Promise<LogEntry> {
  return postSignedMessage(chatId, 'videos', msg)
}

/** Pages forward through a chat's history from sequence `from`. */
export async function fetchHistory(
  chatId: string,
  from: number,
  limit = 50,
): Promise<HistoryResponse> {
  const params = new URLSearchParams({ from: String(from), limit: String(limit) })
  const resp = await fetch(`/chats/${encodeURIComponent(chatId)}/messages?${params}`)
  return parseOrThrow<HistoryResponse>(resp)
}
