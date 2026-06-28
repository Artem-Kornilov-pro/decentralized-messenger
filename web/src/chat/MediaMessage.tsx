import { useEffect, useState } from 'react'
import { base64ToBytes } from '../crypto/base64'
import { decrypt } from '../crypto/aesgcm'
import type { LogEntry } from '../api/types'

interface Props {
  entry: LogEntry
  contentKey: Uint8Array | null
}

type Status = 'loading' | 'ready' | 'no-key' | 'error'

/** Decrypts and renders a photo/video message. Each entry gets its own
 * object URL, revoked on cleanup so decrypted media doesn't pile up in
 * memory as the chat grows. */
export function MediaMessage({ entry, contentKey }: Props) {
  const [status, setStatus] = useState<Status>('loading')
  const [objectUrl, setObjectUrl] = useState<string | null>(null)

  useEffect(() => {
    if (!contentKey) {
      setStatus('no-key')
      return
    }

    setStatus('loading')
    let url: string | null = null
    let cancelled = false

    void decrypt(contentKey, base64ToBytes(entry.message.content))
      .then((plaintext) => {
        if (cancelled) return
        // Same Uint8Array<ArrayBufferLike> vs ArrayBuffer-only BlobPart
        // typing friction as crypto/aesgcm.ts's asBufferSource — not a real
        // runtime concern, plaintext is always a plain ArrayBuffer view.
        url = URL.createObjectURL(
          new Blob([plaintext as BlobPart], { type: entry.message.content_type }),
        )
        setObjectUrl(url)
        setStatus('ready')
      })
      .catch(() => {
        if (!cancelled) setStatus('error')
      })

    return () => {
      cancelled = true
      if (url) URL.revokeObjectURL(url)
    }
  }, [entry.message.content, entry.message.content_type, contentKey])

  if (status === 'no-key') {
    return <span className="media-placeholder">encrypted media — set the content key to view</span>
  }
  if (status === 'error') {
    return <span className="media-placeholder">couldn't decrypt media</span>
  }
  if (status === 'loading' || !objectUrl) {
    return <span className="media-placeholder">decrypting…</span>
  }

  if (entry.message.content_type.startsWith('video/')) {
    return <video src={objectUrl} controls className="media-content" />
  }
  return <img src={objectUrl} alt={entry.message.filename ?? ''} className="media-content" />
}
