'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useUpdateProgress } from '@/hooks/useBooks'
import { usePhysicalCopy, useUpdatePhysicalCopy } from '@/hooks/usePhysicalCopy'
import { useReadingState } from '@/hooks/useReadingState'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import {
  PROGRESS_MODE_PAGES,
  PROGRESS_MODE_PERCENT,
  defaultProgressMode,
  ownsPhysical,
  pageCountOf
} from '@/lib/books/bookProgress'
import {
  continueHint,
  copyDraft,
  savableDraft,
  type PhysicalCopyDraft
} from '@/lib/books/physicalCopy'
import BookContinueHint from '@/components/books/BookContinueHint'
import BookPhysicalCopyFields from '@/components/books/BookPhysicalCopyFields'
import { Button } from '@/components/ui/button'
import { DialogFooter } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'
import { swrKeys } from '@/lib/swrKeys'
import { track } from '@/lib/analytics'

/** Which control opened the progress dialog, sent as the analytics `source`. */
export type BookProgressSource = 'progress_cell' | 'progress_editor'

interface BookProgressFormProps {
  userBook: UserBook
  source?: BookProgressSource
  onSaved?: () => void
  /** Called after save and on Cancel/Escape, so the dialog can close itself. */
  onClose?: () => void
}

const modeOptions = [
  { value: PROGRESS_MODE_PAGES, label: 'Pages' },
  { value: PROGRESS_MODE_PERCENT, label: 'Percent' }
]

/**
 * Reading-progress form (pages/percent), committed via Save or Enter — never
 * on blur: mobile keypads often lack Enter, and blurring into Cancel would
 * save first. Rendered inside `BookProgressDialog`.
 */
export default function BookProgressForm({
  userBook,
  source,
  onSaved,
  onClose
}: BookProgressFormProps) {
  const [progressMode, setProgressMode] = useState(defaultProgressMode(userBook))
  const [currentPage, setCurrentPage] = useState(userBook.currentPage)
  const [progressPercent, setProgressPercent] = useState(userBook.progressPercent)
  const [isSaving, setIsSaving] = useState(false)
  const [draft, setDraft] = useState<PhysicalCopyDraft | null>(null)
  const updateProgress = useUpdateProgress()
  const updatePhysicalCopy = useUpdatePhysicalCopy()
  const isPages = progressMode === PROGRESS_MODE_PAGES
  // A physical copy syncs with the digital one when the book has an EPUB.
  const { data: physical } = usePhysicalCopy(ownsPhysical(userBook) ? userBook.bookId : null)
  const syncable = physical?.syncable ?? false
  const { data: reading } = useReadingState(syncable ? userBook.bookId : null)
  const hint = continueHint(reading?.state)
  const shownDraft = draft ?? copyDraft(physical?.copy)
  const pageCount = (draft?.pageCount ?? 0) || pageCountOf(userBook)

  const handleCommit = async () => {
    if (isSaving) return
    setIsSaving(true)
    try {
      // Details first, so the page maps through them.
      if (draft) await updatePhysicalCopy(userBook.bookId, savableDraft(draft))
      await updateProgress({
        bookId: userBook.bookId,
        progressMode,
        currentPage,
        progressPercent,
        readAt: new Date().toISOString()
      })
      mutate(swrKeys.books)
      if (syncable) mutate(swrKeys.readingState(userBook.bookId))
      track('book_progress_updated', { source, progress_mode: progressMode })
      onSaved?.()
      onClose?.()
    } catch {
      // keep the form open so the user can retry
    } finally {
      setIsSaving(false)
    }
  }

  const handleCancel = () => {
    track('book_progress_dialog_cancelled', { source })
    onClose?.()
    setProgressMode(defaultProgressMode(userBook))
    setCurrentPage(userBook.currentPage)
    setProgressPercent(userBook.progressPercent)
    setDraft(null)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      // Buttons and selects act on Enter themselves.
      if (!(e.target instanceof HTMLInputElement)) return
      e.preventDefault()
      void handleCommit()
    } else if (e.key === 'Escape') {
      handleCancel()
    }
  }

  return (
    <div className="space-y-4" onKeyDown={handleKeyDown}>
      <SegmentedTabs
        aria-label="Progress mode"
        value={progressMode}
        onChange={setProgressMode}
        options={modeOptions}
      />

      <div className="flex items-center gap-3">
        <Input
          key={progressMode}
          type="number"
          inputMode="numeric"
          min={0}
          max={isPages ? undefined : 100}
          value={isPages ? currentPage : progressPercent}
          onChange={(e) =>
            isPages
              ? setCurrentPage(Number(e.target.value))
              : setProgressPercent(Number(e.target.value))
          }
          onFocus={(e) => e.target.select()}
          autoFocus
          aria-label={isPages ? 'Current page' : 'Progress percent'}
          className="h-14 min-w-0 flex-1 text-2xl font-semibold md:text-2xl"
        />
        <span className="shrink-0 text-muted">
          {isPages ? (pageCount > 0 ? `of ${pageCount} pages` : 'pages') : '%'}
        </span>
      </div>

      {isPages && syncable && hint && <BookContinueHint hint={hint} onUse={setCurrentPage} />}

      {isPages && syncable && (
        <BookPhysicalCopyFields
          draft={shownDraft}
          chapters={physical?.chapters ?? []}
          hasPrintPages={physical?.hasPrintPages ?? false}
          onChange={setDraft}
        />
      )}

      <DialogFooter>
        <Button
          variant="secondary"
          onClick={handleCancel}
          disabled={isSaving}
          className="flex-1 sm:flex-none"
        >
          Cancel
        </Button>
        <Button
          onClick={() => void handleCommit()}
          disabled={isSaving}
          className="flex-1 sm:flex-none"
        >
          {isSaving ? 'Saving…' : 'Save'}
        </Button>
      </DialogFooter>
    </div>
  )
}
