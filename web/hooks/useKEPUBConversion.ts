'use client'

import { useEffect, useRef, useState } from 'react'
import { create } from '@bufbuild/protobuf'
import { mutate } from 'swr'
import { useKEPUBStatus, useRequestKEPUBConversion } from '@/hooks/useBooks'
import { GetKEPUBStatusResponseSchema } from '@/lib/gen/books/v1/files_pb'
import { swrKeys } from '@/lib/swrKeys'

export type KEPUBConversion = 'converting' | 'ready' | 'failed'

/**
 * Requests the book's KEPUB on each entry with a non-null `bookId`, then polls
 * until it is ready or failed. Null requests nothing.
 */
export function useKEPUBConversion(bookId: string | null): KEPUBConversion {
  const requestConversion = useRequestKEPUBConversion()
  const requested = useRef<string | null>(null)
  const [answer, setAnswer] = useState<{ bookId: string; failed: boolean } | null>(null)
  const [prevBookId, setPrevBookId] = useState(bookId)
  if (bookId !== prevBookId) {
    setPrevBookId(bookId)
    setAnswer(null)
  }

  useEffect(() => {
    if (requested.current === bookId) return
    requested.current = bookId
    if (!bookId) return
    requestConversion(bookId).then(
      (res) => {
        // Status reads report an outdated KEPUB as ready until its
        // reconversion starts; the request's answer already accounts for it.
        void mutate(
          swrKeys.kepubStatus(bookId),
          create(GetKEPUBStatusResponseSchema, { kepubStatus: res.kepubStatus }),
          { revalidate: false }
        )
        setAnswer({ bookId, failed: false })
      },
      () => setAnswer({ bookId, failed: true })
    )
  }, [bookId, requestConversion])

  // Ignore a late answer for a book switched away from.
  const answered = answer?.bookId === bookId ? answer : null
  const { data, error } = useKEPUBStatus(answered && !answered.failed ? bookId : null)
  if (answered?.failed) return 'failed'
  const status = data?.kepubStatus
  if (status === 'ready') return 'ready'
  // A read error only fails before any status arrives; polling outlives it.
  if (status === 'failed' || (!data && error)) return 'failed'
  return 'converting'
}
