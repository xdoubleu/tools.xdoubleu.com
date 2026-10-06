'use client'

import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/dialog'
import {
  deleteAllStoredBooks,
  listStoredBooks,
  subscribeStoredBooks
} from '@/lib/books/offlineBooks'

export function formatStorageSize(bytes: number): string {
  const mb = bytes / (1024 * 1024)
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`
  if (mb >= 1) return `${mb.toFixed(1)} MB`
  return `${Math.ceil(bytes / 1024)} KB`
}

interface StoredSummary {
  books: number
  bytes: number
}

/** How many books this device keeps for offline reading, and a way to remove them. */
export default function OfflineBooksSettings() {
  const [summary, setSummary] = useState<StoredSummary | null>(null)
  const [confirming, setConfirming] = useState(false)
  const [removing, setRemoving] = useState(false)

  useEffect(() => {
    let active = true
    const load = () =>
      void listStoredBooks().then((files) => {
        if (!active) return
        setSummary({
          books: new Set(files.map((f) => f.bookId)).size,
          bytes: files.reduce((sum, f) => sum + f.size, 0)
        })
      })
    load()
    const unsubscribe = subscribeStoredBooks(load)
    return () => {
      active = false
      unsubscribe()
    }
  }, [])

  async function handleRemove() {
    setRemoving(true)
    await deleteAllStoredBooks()
    setRemoving(false)
    setConfirming(false)
  }

  return (
    <section className="mt-10 border-t border-border pt-8">
      <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
        Offline books
      </h2>
      <p className="mb-3 text-xs text-muted">
        Books you open in the reader, and books you&apos;re currently reading, are kept on this
        device for offline reading. Finished books are removed. On iPhone and iPad they&apos;re kept
        reliably only when this app is added to the Home Screen.
      </p>
      {summary && (
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-sm">
            {summary.books === 0
              ? 'No books stored on this device.'
              : `${summary.books} ${summary.books === 1 ? 'book' : 'books'} stored on this device (${formatStorageSize(summary.bytes)}).`}
          </p>
          {summary.books > 0 && (
            <Button
              type="button"
              variant="destructive"
              size="sm"
              onClick={() => setConfirming(true)}
            >
              Remove offline books
            </Button>
          )}
        </div>
      )}
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title="Remove offline books"
        description="Delete every book stored on this device? Books you're currently reading are downloaded again while you're online."
        confirmLabel="Remove"
        pendingLabel="Removing…"
        destructive
        pending={removing}
        onConfirm={() => void handleRemove()}
      />
    </section>
  )
}
