import { useEffect, useRef, type RefObject } from 'react'
import useSWR from 'swr'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { createKeepaliveClient, createServiceClient } from '@/lib/client'
import { updateReadingProgressWrite } from '@/lib/books/offlineWrites'
import { enqueueWrite } from '@/lib/offline/outbox'
import { swrKeys } from '@/lib/swrKeys'
import {
  LibraryService,
  type GetReadingStateResponse,
  type UpdateReadingProgressRequestSchema
} from '@/lib/gen/books/v1/library_pb'
import type { ReaderLocation } from '@/components/books/reader/BookReader'

/** How long reading pauses before the position is saved. */
export const READING_SAVE_DELAY_MS = 2000

type SaveRequest = MessageInitShape<typeof UpdateReadingProgressRequestSchema>

export function useReadingState(bookId: string | null) {
  const client = createServiceClient(LibraryService)
  return useSWR<GetReadingStateResponse, Error>(bookId ? swrKeys.readingState(bookId) : null, () =>
    client.getReadingState({ bookId: bookId! })
  )
}

/**
 * Returns a relocate handler that saves the reading position once reading
 * pauses, stamped with when it was read. A pending save is flushed when the
 * page is hidden or unloaded, and on unmount. Saves go through the outbox, so
 * offline reading syncs later.
 */
export function useReadingProgressSaver(bookId: string | null) {
  const pending = useRef<SaveRequest | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  useEffect(() => {
    const onVisibility = () => {
      if (document.visibilityState === 'hidden') flushSave(pending, true)
    }
    const onPageHide = () => flushSave(pending, true)
    document.addEventListener('visibilitychange', onVisibility)
    window.addEventListener('pagehide', onPageHide)
    return () => {
      document.removeEventListener('visibilitychange', onVisibility)
      window.removeEventListener('pagehide', onPageHide)
      flushSave(pending)
    }
  }, [])

  return (location: ReaderLocation) => {
    if (!bookId) return
    pending.current = {
      bookId,
      source: 'web',
      percent: Math.round(location.fraction * 100),
      position: location.position,
      readAt: new Date().toISOString()
    }
    clearTimeout(timer.current)
    timer.current = setTimeout(() => flushSave(pending), READING_SAVE_DELAY_MS)
  }
}

// A stale timer finding nothing pending is a no-op, so flushing leaves it.
function flushSave(pending: RefObject<SaveRequest | null>, leaving = false) {
  const req = pending.current
  if (!req) return
  pending.current = null
  // A hidden or unloading page can be frozen or killed before the outbox
  // stores the save, so also send it now; the queued copy's replay is then a
  // no-op (same read_at).
  if (leaving) {
    createKeepaliveClient(LibraryService)
      .updateReadingProgress(req)
      .catch(() => {})
  }
  // Never interrupt reading; a save that can't be queued is superseded by the next.
  enqueueWrite(updateReadingProgressWrite, req).catch(() => {})
}
