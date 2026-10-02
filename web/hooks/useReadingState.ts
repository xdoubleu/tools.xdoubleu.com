import { useEffect, useRef, type RefObject } from 'react'
import useSWR from 'swr'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { createServiceClient } from '@/lib/client'
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
 * page is hidden or unloaded, and on unmount.
 */
export function useReadingProgressSaver(bookId: string | null) {
  const pending = useRef<SaveRequest | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  useEffect(() => {
    const flush = () => flushSave(pending)
    const onVisibility = () => {
      if (document.visibilityState === 'hidden') flush()
    }
    document.addEventListener('visibilitychange', onVisibility)
    window.addEventListener('pagehide', flush)
    return () => {
      document.removeEventListener('visibilitychange', onVisibility)
      window.removeEventListener('pagehide', flush)
      flush()
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
function flushSave(pending: RefObject<SaveRequest | null>) {
  const req = pending.current
  if (!req) return
  pending.current = null
  // A lost save is superseded by the next one; never interrupt reading.
  createServiceClient(LibraryService)
    .updateReadingProgress(req)
    .catch(() => {})
}
