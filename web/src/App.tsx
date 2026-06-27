import { useState } from 'react'
import { ChatView } from './chat/ChatView'
import { IdentityGate } from './identity/IdentityGate'
import { useIdentity } from './identity/useIdentity'
import './App.css'

function App() {
  const { identity, createIdentity, resetIdentity } = useIdentity()
  const [chatId, setChatId] = useState('')
  const [joinedChatId, setJoinedChatId] = useState<string | null>(null)

  if (!identity) {
    return <IdentityGate onCreate={createIdentity} />
  }

  return (
    <div className="app">
      <header className="topbar">
        <span>
          {identity.senderId} <button type="button" onClick={resetIdentity}>reset identity</button>
        </span>
      </header>

      {!joinedChatId ? (
        <form
          className="join-chat"
          onSubmit={(e) => {
            e.preventDefault()
            const trimmed = chatId.trim()
            if (trimmed) setJoinedChatId(trimmed)
          }}
        >
          <input
            value={chatId}
            onChange={(e) => setChatId(e.target.value)}
            placeholder="chat ID (anything you and the other side agree on)"
            autoFocus
          />
          <button type="submit" disabled={chatId.trim() === ''}>
            Join
          </button>
        </form>
      ) : (
        <ChatView chatId={joinedChatId} identity={identity} />
      )}
    </div>
  )
}

export default App
