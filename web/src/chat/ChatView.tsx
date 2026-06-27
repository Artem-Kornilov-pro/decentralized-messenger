import { useState } from 'react'
import { useChat } from './useChat'
import { base64ToBytes } from '../crypto/base64'
import type { Identity } from '../identity/types'

interface Props {
  chatId: string
  identity: Identity
}

export function ChatView({ chatId, identity }: Props) {
  const { messages, caughtUp, error, loadMore, send } = useChat(chatId, identity)
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)

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

  return (
    <div className="chat-view">
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
            <span className="text">{decodeText(entry.message.content)}</span>
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
