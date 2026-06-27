/** Mirrors models.SignedMessage as the server returns it (binary fields are
 * base64 strings on the wire). */
export interface SignedMessage {
  schema_version: number
  message_id: string
  chat_id: string
  sender_id: string
  content: string
  content_type: string
  filename?: string
  encrypted: boolean
  timestamp: string
  public_key: string
  signature: string
}

/** Mirrors models.LogEntry. */
export interface LogEntry {
  sequence: number
  message: SignedMessage
  prev_hash: string
  entry_hash: string
}

/** Mirrors api.historyResponse. */
export interface HistoryResponse {
  messages: LogEntry[]
  next_from: number | null
}

/** Mirrors broker.Event, pushed over /chats/{chatID}/ws. */
export interface StreamEvent {
  kind: 'entry_appended' | 'snapshot_created'
  chat_id: string
  sequence: number
  entry_hash: string
}

export interface ApiError {
  error: string
}
