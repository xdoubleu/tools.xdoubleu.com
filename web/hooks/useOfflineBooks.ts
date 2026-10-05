import { useEffect, useRef, useState, useSyncExternalStore } from 'react'
import { listStoredBooks, openBookFile, subscribeStoredBooks } from '@/lib/books/offlineBooks'

interface OfflineBookFile {
  file?: File
  error?: Error
}

/**
 * Opens a book's file, stored copy first (see `openBookFile`). `version` is
 * read once per book and format, so a library refetch never reopens the book.
 */
export function useOfflineBookFile(
  bookId: string | null,
  format: string | null,
  version: string
): OfflineBookFile {
  const key = bookId && format ? `${bookId}:${format}` : null
  const [result, setResult] = useState<OfflineBookFile & { key?: string }>({})
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
  }, [bookId, format])

  if (!key || result.key !== key) return {}
  return result.error ? { error: result.error } : { file: result.file }
}

const NONE: ReadonlySet<string> = new Set()
let storedIds = NONE
let watching = false
const idListeners = new Set<() => void>()

function loadStoredIds() {
  void listStoredBooks().then((files) => {
    storedIds = new Set(files.map((f) => f.bookId))
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
