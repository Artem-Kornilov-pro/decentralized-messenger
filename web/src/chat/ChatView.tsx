import { useRef, useState } from 'react'
import { useChat } from './useChat'
import { useContentKey } from './useContentKey'
import { ContentKeyPanel } from './ContentKeyPanel'
import { MediaMessage } from './MediaMessage'
import { base64ToBytes } from '../crypto/base64'
import type { Identity } from '../identity/types'

interface Props {
  chatId: string
  identity: Identity
}

function isMedia(contentType: string): boolean {
  return contentType.startsWith('image/') || contentType.startsWith('video/')
}

export function ChatView({ chatId, identity }: Props) {
  const { messages, caughtUp, error, loadMore, send, sendAttachment } = useChat(chatId, identity)
  const { contentKey, generate, setFromBase64 } = useContentKey(chatId)
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    const text = draft.trim()
    if (!text) return
    setSending(true)
    try {
      await send(text)
      setDraft('')
    } finally {
      setSending(false)
    }
  }

  const attach = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file || !contentKey) return
    await sendAttachment(file, contentKey)
  }

  return (
    <div className="chat-view">
      <ContentKeyPanel contentKey={contentKey} onGenerate={generate} onSetFromBase64={setFromBase64} />

      <div className="chat-status">
        {caughtUp ? 'live' : 'loading history…'}
        {error && <span className="chat-error"> — {error}</span>}
      </div>

      {!caughtUp && (
        <button type="button" onClick={() => void loadMore()}>
          Load more
        </button>
      )}

      <ul className="message-list">
        {messages.map((entry) => (
          <li key={entry.sequence} className={entry.message.sender_id === identity.senderId ? 'mine' : ''}>
            <span className="sender">{entry.message.sender_id}</span>
            {isMedia(entry.message.content_type) ? (
              <MediaMessage entry={entry} contentKey={contentKey} />
            ) : (
              <span className="text">{decodeText(entry.message.content)}</span>
            )}
          </li>
        ))}
      </ul>

      <form className="composer" onSubmit={submit}>
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          placeholder="Type a message"
          autoFocus
        />
        <button type="submit" disabled={sending || draft.trim() === ''}>
          Send
        </button>
        <button
          type="button"
          disabled={!contentKey}
          title={contentKey ? 'Attach a photo or video' : 'Set a content key first'}
          onClick={() => fileInputRef.current?.click()}
        >
          Attach
        </button>
        <input
          ref={fileInputRef}
          type="file"
          accept="image/*,video/*"
          onChange={(e) => void attach(e)}
          hidden
        />
      </form>
    </div>
  )
}

function decodeText(base64Content: string): string {
  try {
    return new TextDecoder().decode(base64ToBytes(base64Content))
  } catch {
    return '(unreadable)'
  }
}
