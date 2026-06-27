import { useState } from 'react'

interface Props {
  onCreate: (senderId: string) => Promise<unknown>
}

/** Shown when no local identity exists: pick a display name, generate an
 * Ed25519 key pair, persist it locally. The private key never leaves this
 * browser. */
export function IdentityGate({ onCreate }: Props) {
  const [senderId, setSenderId] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = senderId.trim()
    if (!trimmed) return
    setBusy(true)
    try {
      await onCreate(trimmed)
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="identity-gate" onSubmit={submit}>
      <h1>decentralized-messenger</h1>
      <p>Pick a display name. An Ed25519 key pair is generated and stored only in this browser.</p>
      <input
        value={senderId}
        onChange={(e) => setSenderId(e.target.value)}
        placeholder="display name"
        autoFocus
      />
      <button type="submit" disabled={busy || senderId.trim() === ''}>
        {busy ? 'Generating…' : 'Generate identity'}
      </button>
    </form>
  )
}
