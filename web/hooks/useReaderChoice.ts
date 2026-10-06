'use client'

import { useState, useSyncExternalStore } from 'react'
import {
  loadReaderChoice,
  requestedReaderChoice,
  saveReaderChoice,
  type ReaderChoice
} from '@/lib/books/readerChoice'

// Only a toggle in this tab changes the choice while reading.
// Stryker disable next-line ArrowFunction: equivalent; React skips a missing unsubscribe.
const subscribe = () => () => {}

/**
 * The reader's choice for a book: a toggle in this visit, then `?format=`,
 * then this device's stored choice, then the original. Undefined while
 * server-rendering and hydrating, before storage can be read.
 */
export function useReaderChoice(
  bookId: string | null,
  requested: string | null
): [ReaderChoice | undefined, (choice: ReaderChoice) => void] {
  const [toggled, setToggled] = useState<ReaderChoice | null>(null)
  const stored = useSyncExternalStore(
    subscribe,
    () => (bookId ? loadReaderChoice(bookId) : null),
    () => undefined
  )
  const choice =
    toggled ??
    requestedReaderChoice(requested) ??
    (stored === undefined ? undefined : (stored ?? 'original'))

  const choose = (next: ReaderChoice) => {
    setToggled(next)
    if (bookId) saveReaderChoice(bookId, next)
  }
  return [choice, choose]
}
