import { useState } from 'react'
import { bytesToBase64 } from '../crypto/base64'

interface Props {
  contentKey: Uint8Array | null
  onGenerate: () => void
  onSetFromBase64: (b64: string) => void
}

/** Manages the chat's photo/video encryption key: shows the current key for
 * copying, accepts one pasted from another participant, or generates a new
 * one. The server never sees this key — share it out of band. */
export function ContentKeyPanel({ contentKey, onGenerate, onSetFromBase64 }: Props) {
  const [pasted, setPasted] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const currentB64 = contentKey ? bytesToBase64(contentKey) : ''

  const copy = async () => {
    await navigator.clipboard.writeText(currentB64)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const usePasted = () => {
    setError(null)
    try {
      onSetFromBase64(pasted)
      setPasted('')
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className="content-key-panel">
      {contentKey ? (
        <div className="content-key-current">
          <input readOnly value={currentB64} aria-label="content key" />
          <button type="button" onClick={() => void copy()}>
            {copied ? 'Copied' : 'Copy'}
          </button>
        </div>
      ) : (
        <p className="content-key-missing">
          No content key set for this chat — photos/videos can't be sent or viewed until one is.
        </p>
      )}

      <div className="content-key-paste">
        <input
          value={pasted}
          onChange={(e) => setPasted(e.target.value)}
          placeholder="paste a key from another participant"
        />
        <button type="button" onClick={usePasted} disabled={pasted.trim() === ''}>
          Use
        </button>
        <button type="button" onClick={onGenerate}>
          Generate new key
        </button>
      </div>
      {error && <p className="content-key-error">{error}</p>}
    </div>
  )
}
