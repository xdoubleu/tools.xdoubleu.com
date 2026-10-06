import { useEffect, useRef, useState, useSyncExternalStore } from 'react'
import {
  openBookFile,
  storedBookIds,
  storedBookVersion,
  subscribeStoredBooks
} from '@/lib/books/offlineBooks'

interface OfflineBookFile {
  file?: File
  error?: Error
}

/**
 * Opens a book's file, stored copy first (see `openBookFile`), retrying a
 * failed open when the connection returns. `version` is read once per book
 * and format, so a library refetch never reopens the book.
 */
export function useOfflineBookFile(
  bookId: string | null,
  format: string | null,
  version: string
): OfflineBookFile {
  const key = bookId && format ? `${bookId}:${format}` : null
  const [result, setResult] = useState<OfflineBookFile & { key?: string }>({})
  const [attempt, setAttempt] = useState(0)
  const versionRef = useRef(version)
  useEffect(() => {
    versionRef.current = version
  }, [version])

  useEffect(() => {
    if (!bookId || !format) return
    let cancelled = false
    const key = `${bookId}:${format}`
    openBookFile({ bookId, format, version: versionRef.current }).then(
      (file) => !cancelled && setResult({ key, file }),
      (err: unknown) =>
        !cancelled && setResult({ key, error: err instanceof Error ? err : new Error(String(err)) })
    )
    return () => {
      cancelled = true
    }
  }, [bookId, format, attempt])

  const failed = result.error !== undefined
  useEffect(() => {
    if (!failed) return
    const retry = () => setAttempt((n) => n + 1)
    window.addEventListener('online', retry)
    return () => window.removeEventListener('online', retry)
  }, [failed])

  if (!key || result.key !== key) return {}
  return result.error ? { error: result.error } : { file: result.file }
}

const NONE: ReadonlySet<string> = new Set()
let storedIds = NONE
let watching = false
const idListeners = new Set<() => void>()

function loadStoredIds() {
  void storedBookIds().then((ids) => {
    storedIds = ids
    idListeners.forEach((l) => l())
  })
}

function subscribeStoredIds(listener: () => void) {
  idListeners.add(listener)
  if (!watching) {
    watching = true
    subscribeStoredBooks(loadStoredIds)
    loadStoredIds()
  }
  return () => {
    idListeners.delete(listener)
  }
}

/** IDs of books with a file stored for offline reading; one listing shared by every caller. */
export function useStoredBookIds(): ReadonlySet<string> {
  return useSyncExternalStore(
    subscribeStoredIds,
    () => storedIds,
    () => NONE
  )
}

/**
 * The version of the book's stored `format`: undefined while checking, null
 * when none is stored (or no book is given).
 */
export function useStoredBookVersion(
  bookId: string | null,
  format: string
): string | null | undefined {
  const key = bookId ? `${bookId}:${format}` : null
  const [stored, setStored] = useState<{ key: string; version: string | null } | null>(null)
  useEffect(() => {
    if (!bookId) return
    let active = true
    const key = `${bookId}:${format}`
    const load = () =>
      void storedBookVersion(bookId, format).then((version) => {
        if (active) setStored({ key, version })
      })
    load()
    const unsubscribe = subscribeStoredBooks(load)
    return () => {
      active = false
      unsubscribe()
    }
  }, [bookId, format])
  if (!key) return null
  return stored?.key === key ? stored.version : undefined
}
