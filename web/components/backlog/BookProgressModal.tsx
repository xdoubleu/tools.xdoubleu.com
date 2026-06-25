'use client'

import { useState } from 'react'
import { useUpdateProgress } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import {
  PROGRESS_MODE_PAGES,
  PROGRESS_MODE_PERCENT,
  defaultProgressMode
} from '@/lib/backlog/bookProgress'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'

interface BookProgressModalProps {
  userBook: UserBook
  onClose: () => void
  onSaved: () => void
}

export default function BookProgressModal({ userBook, onClose, onSaved }: BookProgressModalProps) {
  const [progressMode, setProgressMode] = useState(defaultProgressMode(userBook))
  const [currentPage, setCurrentPage] = useState(userBook.currentPage)
  const [progressPercent, setProgressPercent] = useState(userBook.progressPercent)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const updateProgress = useUpdateProgress()

  const book = userBook.book

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsSubmitting(true)
    setError(null)
    try {
      await updateProgress({
        bookId: userBook.id,
        progressMode,
        currentPage,
        progressPercent
      })
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update progress.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{book?.title ?? 'Update progress'}</DialogTitle>
          <DialogClose>×</DialogClose>
        </DialogHeader>
        {book && book.authors.length > 0 && (
          <p className="mb-4 text-sm text-muted">{book.authors.join(', ')}</p>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="progress-mode">Progress</Label>
            <div className="flex gap-2">
              <Select
                id="progress-mode"
                value={progressMode}
                onChange={(e) => setProgressMode(e.target.value)}
                className="w-32"
              >
                <option value={PROGRESS_MODE_PAGES}>Pages</option>
                <option value={PROGRESS_MODE_PERCENT}>Percent</option>
              </Select>
              {progressMode === PROGRESS_MODE_PAGES ? (
                <div className="flex flex-1 items-center gap-2">
                  <Input
                    type="number"
                    min={0}
                    value={currentPage}
                    onChange={(e) => setCurrentPage(Number(e.target.value))}
                    aria-label="Current page"
                  />
                  {book?.pageCount ? (
                    <span className="text-sm text-muted whitespace-nowrap">/ {book.pageCount}</span>
                  ) : null}
                </div>
              ) : (
                <div className="flex flex-1 items-center gap-2">
                  <Input
                    type="number"
                    min={0}
                    max={100}
                    value={progressPercent}
                    onChange={(e) => setProgressPercent(Number(e.target.value))}
                    aria-label="Progress percent"
                  />
                  <span className="text-sm text-muted">%</span>
                </div>
              )}
            </div>
          </div>

          {error && <p className="text-sm text-danger">{error}</p>}

          <div className="flex gap-2 pt-2">
            <Button type="submit" disabled={isSubmitting} className="flex-1">
              {isSubmitting ? 'Saving...' : 'Save'}
            </Button>
            <Button type="button" variant="secondary" onClick={onClose} className="flex-1">
              Cancel
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
