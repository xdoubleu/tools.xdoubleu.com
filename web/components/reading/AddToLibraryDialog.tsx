'use client'

import { useState } from 'react'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import BookSearchBar from '@/components/reading/BookSearchBar'
import AddByUrlForm from '@/components/reading/AddByUrlForm'
import AddFeedForm from '@/components/reading/AddFeedForm'

type Mode = 'book' | 'url' | 'rss'

const MODES: { id: Mode; label: string }[] = [
  { id: 'book', label: 'Book' },
  { id: 'url', label: 'By URL' },
  { id: 'rss', label: 'RSS feed' }
]

// AddToLibraryDialog is the single entry point for adding to the reading
// library: search for a book, paste a paper/article URL, or subscribe to an
// RSS feed — replacing the previously separate add flows.
export default function AddToLibraryDialog({
  open,
  onOpenChange,
  onAdded
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onAdded?: () => void
}) {
  const [mode, setMode] = useState<Mode>('book')

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add to library</DialogTitle>
        </DialogHeader>

        <div
          role="tablist"
          aria-label="What to add"
          className="mb-4 flex gap-1 rounded-xl border border-border bg-surface p-1"
        >
          {MODES.map((m) => (
            <Button
              key={m.id}
              role="tab"
              aria-selected={mode === m.id}
              size="sm"
              variant={mode === m.id ? 'default' : 'ghost'}
              className="flex-1"
              onClick={() => setMode(m.id)}
            >
              {m.label}
            </Button>
          ))}
        </div>

        {mode === 'book' && (
          <div>
            <p className="mb-2 text-sm text-muted">Search for a book to add to your library.</p>
            <BookSearchBar onAdded={() => onAdded?.()} />
          </div>
        )}
        {mode === 'url' && <AddByUrlForm onAdded={onAdded} onDone={() => onOpenChange(false)} />}
        {mode === 'rss' && <AddFeedForm onAdded={onAdded} />}
      </DialogContent>
    </Dialog>
  )
}
