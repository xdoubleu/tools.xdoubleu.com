'use client'

import { useState } from 'react'
import { useUpdateBookStatus, useToggleTag, useUpdateProgress } from '@/hooks/useBacklog'
import type { UpdateBookStatusInput } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import { PROGRESS_MODE_PAGES, PROGRESS_MODE_PERCENT } from '@/lib/backlog/bookProgress'
import KoboSyncToggle from '@/components/backlog/KoboSyncToggle'
import BookPreviewDialog from '@/components/backlog/BookPreviewDialog'
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
import { Textarea } from '@/components/ui/textarea'

const BOOK_STATUSES = ['wishlist', 'reading', 'finished', 'dnf']

interface BookEditModalProps {
  userBook: UserBook
  onClose: () => void
  onSaved: () => void
}

export default function BookEditModal({ userBook, onClose, onSaved }: BookEditModalProps) {
  const [status, setStatus] = useState(userBook.status)
  const [rating, setRating] = useState(userBook.rating)
  const [notes, setNotes] = useState(userBook.notes)
  const [favourite, setFavourite] = useState(userBook.tags.includes('favourite'))
  const [ownPhysical, setOwnPhysical] = useState(userBook.tags.includes('own-physical'))
  const [ownDigital, setOwnDigital] = useState(userBook.tags.includes('own-digital'))
  const [progressMode, setProgressMode] = useState(userBook.progressMode || PROGRESS_MODE_PAGES)
  const [currentPage, setCurrentPage] = useState(userBook.currentPage)
  const [progressPercent, setProgressPercent] = useState(userBook.progressPercent)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [previewFormat, setPreviewFormat] = useState<'pdf' | 'epub' | 'kepub' | null>(null)
  const updateBookStatus = useUpdateBookStatus()
  const updateProgress = useUpdateProgress()
  const toggleTag = useToggleTag()

  const book = userBook.book

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsSubmitting(true)
    setError(null)
    try {
      const currentOwnPhysical = userBook.tags.includes('own-physical')
      const currentOwnDigital = userBook.tags.includes('own-digital')
      const req: UpdateBookStatusInput = {
        bookId: userBook.id,
        status,
        rating: String(rating),
        notes,
        favourite
      }
      await Promise.all([
        updateBookStatus(req),
        updateProgress({
          bookId: userBook.id,
          progressMode,
          currentPage,
          progressPercent
        }),
        ownPhysical !== currentOwnPhysical && toggleTag(userBook.id, 'own-physical'),
        ownDigital !== currentOwnDigital && toggleTag(userBook.id, 'own-digital')
      ])
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update book.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <>
      <Dialog open onOpenChange={(open) => !open && onClose()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{book?.title ?? 'Edit Book'}</DialogTitle>
            <DialogClose>×</DialogClose>
          </DialogHeader>
          {book && book.authors.length > 0 && (
            <p className="mb-4 text-sm text-muted">{book.authors.join(', ')}</p>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="edit-status">Status</Label>
              <Select id="edit-status" value={status} onChange={(e) => setStatus(e.target.value)}>
                {BOOK_STATUSES.map((s) => (
                  <option key={s} value={s}>
                    {s.charAt(0).toUpperCase() + s.slice(1)}
                  </option>
                ))}
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="edit-rating">Rating (0 = unrated)</Label>
              <Select
                id="edit-rating"
                value={rating}
                onChange={(e) => setRating(Number(e.target.value))}
              >
                {[0, 1, 2, 3, 4, 5].map((r) => (
                  <option key={r} value={r}>
                    {r === 0 ? 'No rating' : `${r} star${r > 1 ? 's' : ''}`}
                  </option>
                ))}
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="edit-progress-mode">Progress</Label>
              <div className="flex gap-2">
                <Select
                  id="edit-progress-mode"
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
                      <span className="text-sm text-muted whitespace-nowrap">
                        / {book.pageCount}
                      </span>
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

            <div className="space-y-1.5">
              <Label htmlFor="edit-notes">Notes</Label>
              <Textarea
                id="edit-notes"
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                rows={3}
                placeholder="Optional notes..."
                className="resize-none"
              />
            </div>

            <div className="flex flex-wrap gap-4">
              {[
                {
                  id: 'edit-favourite',
                  state: favourite,
                  setter: setFavourite,
                  label: 'Favourite'
                },
                {
                  id: 'edit-own-physical',
                  state: ownPhysical,
                  setter: setOwnPhysical,
                  label: 'Own physical'
                },
                {
                  id: 'edit-own-digital',
                  state: ownDigital,
                  setter: setOwnDigital,
                  label: 'Own digital'
                }
              ].map(({ id, state, setter, label }) => (
                <div key={id} className="flex items-center gap-2">
                  <input
                    id={id}
                    type="checkbox"
                    checked={state}
                    onChange={(e) => setter(e.target.checked)}
                    className="rounded accent-[rgb(var(--color-accent))]"
                  />
                  <label htmlFor={id} className="text-sm text-subtle cursor-pointer">
                    {label}
                  </label>
                </div>
              ))}
            </div>

            <KoboSyncToggle
              bookId={userBook.bookId}
              enabled={userBook.tags.includes('kobo-sync')}
              tags={userBook.tags}
              onChanged={onSaved}
            />

            {(userBook.formats.includes('pdf') || userBook.formats.includes('epub')) && (
              <div className="space-y-1.5">
                <p className="text-xs text-muted font-medium">Preview</p>
                <div className="flex gap-2 flex-wrap">
                  {userBook.formats.includes('pdf') && (
                    <Button
                      type="button"
                      variant="secondary"
                      className="text-xs"
                      onClick={() => setPreviewFormat('pdf')}
                    >
                      Preview PDF
                    </Button>
                  )}
                  {userBook.formats.includes('epub') ? (
                    <Button
                      type="button"
                      variant="secondary"
                      className="text-xs"
                      onClick={() => setPreviewFormat('epub')}
                    >
                      Preview EPUB
                    </Button>
                  ) : (
                    userBook.formats.includes('pdf') && (
                      <Button
                        type="button"
                        variant="secondary"
                        className="text-xs"
                        onClick={() => setPreviewFormat('kepub')}
                      >
                        Preview EPUB
                      </Button>
                    )
                  )}
                </div>
              </div>
            )}

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

      {previewFormat && (
        <BookPreviewDialog
          bookId={userBook.bookId}
          format={previewFormat}
          title={book?.title ?? 'Book Preview'}
          open={!!previewFormat}
          onOpenChange={(open) => !open && setPreviewFormat(null)}
        />
      )}
    </>
  )
}
