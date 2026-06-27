import { useCallback, useState } from 'react'
import { generateKeyPair } from '../crypto/ed25519'
import { clearIdentity, loadIdentity, saveIdentity } from './storage'
import type { Identity } from './types'

export function useIdentity() {
  const [identity, setIdentity] = useState<Identity | null>(() => loadIdentity())

  const createIdentity = useCallback(async (senderId: string) => {
    const { publicKey, secretKey } = await generateKeyPair()
    const next: Identity = { senderId, publicKey, secretKey }
    saveIdentity(next)
    setIdentity(next)
    return next
  }, [])

  const resetIdentity = useCallback(() => {
    clearIdentity()
    setIdentity(null)
  }, [])

  return { identity, createIdentity, resetIdentity }
}
