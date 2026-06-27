import { useCallback, useEffect, useRef, useState } from 'react'
import { fetchHistory, sendText } from '../api/client'
import { openStream } from '../api/stream'
import type { LogEntry } from '../api/types'
import { sign } from '../crypto/ed25519'
import { CONTENT_TYPE_TEXT, newMessage, signingPayload, toWireFormat } from '../crypto/signedMessage'
import type { Identity } from '../identity/types'

const PAGE_SIZE = 50

export function useChat(chatId: string, identity: Identity) {
  const [messages, setMessages] = useState<LogEntry[]>([])
  const [caughtUp, setCaughtUp] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // Tracks the next sequence to fetch, across renders and across the async
  // gap between "load a page" and "a live WS event arrives".
  const nextSeqRef = useRef(0)

  // Dedupes by sequence: an optimistic append from send() and the
  // corresponding WS-triggered fetch can race and both deliver the same entry.
  const appendEntries = useCallback((entries: LogEntry[]) => {
    if (entries.length === 0) return
    setMessages((prev) => {
      const seen = new Set(prev.map((e) => e.sequence))
      const fresh = entries.filter((e) => !seen.has(e.sequence))
      return fresh.length ? [...prev, ...fresh] : prev
    })
    nextSeqRef.current = Math.max(
      nextSeqRef.current,
      entries[entries.length - 1].sequence + 1,
    )
  }, [])

  // Forward-paginate through history (the API's only pagination mode) until
  // next_from is null, i.e. caught up to the live tip.
  const loadMore = useCallback(async () => {
    try {
      const page = await fetchHistory(chatId, nextSeqRef.current, PAGE_SIZE)
      appendEntries(page.messages)
      setCaughtUp(page.next_from === null)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }, [chatId, appendEntries])

  useEffect(() => {
    setMessages([])
    setCaughtUp(false)
    setError(null)
    nextSeqRef.current = 0
    void loadMore()
    // chatId change alone should reset and reload; loadMore is re-created
    // per chatId via its own dependency array.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chatId])

  // Once caught up to the tip, switch to live updates: any entry_appended
  // event for this chat means "fetch what's new since our last sequence".
  useEffect(() => {
    if (!caughtUp) return
    const close = openStream(chatId, (evt) => {
      if (evt.chat_id !== chatId || evt.kind !== 'entry_appended') return
      void fetchHistory(chatId, nextSeqRef.current, PAGE_SIZE).then((page) =>
        appendEntries(page.messages),
      )
    })
    return close
  }, [chatId, caughtUp, appendEntries])

  const send = useCallback(
    async (text: string) => {
      try {
        const content = new TextEncoder().encode(text)
        const msg = newMessage(
          chatId,
          identity.senderId,
          identity.publicKey,
          content,
          CONTENT_TYPE_TEXT,
          '',
          false,
        )
        const signature = await sign(signingPayload(msg), identity.secretKey)
        const entry = await sendText(chatId, toWireFormat(msg, signature))
        appendEntries([entry])
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err))
      }
    },
    [chatId, identity, appendEntries],
  )

  return { messages, caughtUp, error, loadMore, send }
}
