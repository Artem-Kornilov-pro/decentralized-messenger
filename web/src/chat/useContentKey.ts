import { useCallback, useEffect, useState } from 'react'
import { base64ToBytes } from '../crypto/base64'
import { generateContentKey } from '../crypto/aesgcm'
import { clearContentKey, loadContentKey, saveContentKey } from './contentKey'

export function useContentKey(chatId: string) {
  const [contentKey, setContentKey] = useState<Uint8Array | null>(() => loadContentKey(chatId))

  // Reload (don't carry a key from a previously joined chat) whenever the
  // chat changes.
  useEffect(() => {
    setContentKey(loadContentKey(chatId))
  }, [chatId])

  const generate = useCallback(() => {
    const key = generateContentKey()
    saveContentKey(chatId, key)
    setContentKey(key)
    return key
  }, [chatId])

  /** Adopts a key pasted from another participant. Throws if it isn't
   * valid base64 / the right length, leaving the current key untouched. */
  const setFromBase64 = useCallback(
    (b64: string) => {
      const key = base64ToBytes(b64.trim())
      if (key.length !== 32) {
        throw new Error('content key must decode to 32 bytes')
      }
      saveContentKey(chatId, key)
      setContentKey(key)
    },
    [chatId],
  )

  const clear = useCallback(() => {
    clearContentKey(chatId)
    setContentKey(null)
  }, [chatId])

  return { contentKey, generate, setFromBase64, clear }
}
