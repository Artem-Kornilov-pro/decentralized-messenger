// Reproduces internal/models/message.go's SignedMessage and SigningPayload
// byte-for-byte. The signature covers exactly these bytes, so any mismatch
// here makes every message the frontend sends fail verification.
import { bytesToBase64, utf8ToBytes } from './base64'
import { formatTimestamp } from './timestamp'

export const SCHEMA_VERSION = 1
export const CONTENT_TYPE_TEXT = 'text/plain'

export interface UnsignedMessage {
  schemaVersion: number
  messageId: string
  chatId: string
  senderId: string
  content: Uint8Array
  contentType: string
  filename: string
  encrypted: boolean
  /** Already formatted as Go's RFC3339Nano would render it (see formatTimestamp). */
  timestamp: string
  publicKey: Uint8Array
}

/** The wire shape POSTed to /chats/{chatID}/messages|photos|videos. */
export interface SignedMessageJSON {
  schema_version: number
  message_id: string
  chat_id: string
  sender_id: string
  content: string
  content_type: string
  filename: string
  encrypted: boolean
  timestamp: string
  public_key: string
  signature: string
}

export function newMessageId(): string {
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
}

/** Mirrors models.NewMessage: builds an unsigned message ready to sign. */
export function newMessage(
  chatId: string,
  senderId: string,
  publicKey: Uint8Array,
  content: Uint8Array,
  contentType: string,
  filename: string,
  encrypted: boolean,
): UnsignedMessage {
  return {
    schemaVersion: SCHEMA_VERSION,
    messageId: newMessageId(),
    chatId,
    senderId,
    content,
    contentType,
    filename,
    encrypted,
    timestamp: formatTimestamp(new Date()),
    publicKey,
  }
}

// Characters Go's json.Marshal always escapes, regardless of SetEscapeHTML —
// see encoding/json's HTMLEscape and internal/models/message.go's comment on
// SigningPayload's determinism. Built from charCodes (rather than literal
// source characters) so the line/paragraph separators can't end up as raw
// control characters in this file.
const GO_ESCAPES: [string, string][] = [
  ['<', '\\u003c'],
  ['>', '\\u003e'],
  ['&', '\\u0026'],
  [String.fromCharCode(0x2028), '\\u2028'],
  [String.fromCharCode(0x2029), '\\u2029'],
]

function goEscape(jsonText: string): string {
  let out = jsonText
  for (const [from, to] of GO_ESCAPES) {
    out = out.split(from).join(to)
  }
  return out
}

/** Encodes a map[string]string the way Go's json.Marshal does: keys sorted
 * lexicographically, no insignificant whitespace, HTML-sensitive characters
 * escaped. */
function canonicalJSON(fields: Record<string, string>): string {
  const keys = Object.keys(fields).sort()
  const parts = keys.map((k) => `${JSON.stringify(k)}:${JSON.stringify(fields[k])}`)
  return goEscape(`{${parts.join(',')}}`)
}

/** Reproduces SignedMessage.SigningPayload(). */
export function signingPayload(m: UnsignedMessage): Uint8Array {
  const fields: Record<string, string> = {
    schema_version: String(m.schemaVersion),
    message_id: m.messageId,
    chat_id: m.chatId,
    sender_id: m.senderId,
    content: bytesToBase64(m.content),
    content_type: m.contentType,
    filename: m.filename,
    encrypted: m.encrypted ? 'true' : 'false',
    timestamp: m.timestamp,
    public_key: bytesToBase64(m.publicKey),
  }
  return utf8ToBytes(canonicalJSON(fields))
}

/** Builds the JSON body to POST, given a message and its signature. */
export function toWireFormat(m: UnsignedMessage, signature: Uint8Array): SignedMessageJSON {
  return {
    schema_version: m.schemaVersion,
    message_id: m.messageId,
    chat_id: m.chatId,
    sender_id: m.senderId,
    content: bytesToBase64(m.content),
    content_type: m.contentType,
    filename: m.filename,
    encrypted: m.encrypted,
    timestamp: m.timestamp,
    public_key: bytesToBase64(m.publicKey),
    signature: bytesToBase64(signature),
  }
}
