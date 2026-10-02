'use client'

import { useEffect } from 'react'
import { flushOutbox, getOutboxSnapshot } from '@/lib/offline/outbox'

const RETRY_MS = 30_000

/** Sends queued writes on load, on reconnect, and periodically while any wait. */
export default function OutboxSync() {
  useEffect(() => {
    const flush = () => void flushOutbox()
    flush()
    window.addEventListener('online', flush)
    const id = setInterval(() => {
      if (getOutboxSnapshot().pending > 0) flush()
    }, RETRY_MS)
    return () => {
      window.removeEventListener('online', flush)
      clearInterval(id)
    }
  }, [])

  return null
}
