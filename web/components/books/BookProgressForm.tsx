'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useUpdateProgress } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import {
  PROGRESS_MODE_PAGES,
  PROGRESS_MODE_PERCENT,
  defaultProgressMode
} from '@/lib/books/bookProgress'
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
  const updateProgress = useUpdateProgress()
  const pageCount = userBook.book?.pageCount ?? 0
  const isPages = progressMode === PROGRESS_MODE_PAGES

  const handleCommit = async () => {
    if (isSaving) return
    setIsSaving(true)
    try {
      await updateProgress({
        bookId: userBook.bookId,
        progressMode,
        currentPage,
        progressPercent
      })
      mutate(swrKeys.books)
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
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
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
