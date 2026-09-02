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
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { swrKeys } from '@/lib/swrKeys'

interface BookProgressFormProps {
  userBook: UserBook
  onSaved?: () => void
  /** Called after a successful save, and on Escape — lets an embedding popover/toggle close itself. */
  onClose?: () => void
}

/**
 * The reading-progress edit form: a mode select (pages/percent) plus a
 * numeric input, committing on blur or Enter. Shared between the card view's
 * click-to-toggle usage (`BookProgressEditor`) and the library table's
 * "Progress" column popover (`BookProgressCell`).
 */
export default function BookProgressForm({ userBook, onSaved, onClose }: BookProgressFormProps) {
  const [progressMode, setProgressMode] = useState(defaultProgressMode(userBook))
  const [currentPage, setCurrentPage] = useState(userBook.currentPage)
  const [progressPercent, setProgressPercent] = useState(userBook.progressPercent)
  const [isSaving, setIsSaving] = useState(false)
  const updateProgress = useUpdateProgress()

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
      onSaved?.()
      onClose?.()
    } catch {
      // keep the form open so the user can retry
    } finally {
      setIsSaving(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      void handleCommit()
    } else if (e.key === 'Escape') {
      onClose?.()
      // reset to stored values
      setProgressMode(defaultProgressMode(userBook))
      setCurrentPage(userBook.currentPage)
      setProgressPercent(userBook.progressPercent)
    }
  }

  return (
    <div className="space-y-1.5" onKeyDown={handleKeyDown}>
      <div className="flex gap-2 items-center">
        <Select
          value={progressMode}
          onChange={(e) => setProgressMode(e.target.value)}
          className="w-28"
          aria-label="Progress mode"
        >
          <option value={PROGRESS_MODE_PAGES}>Pages</option>
          <option value={PROGRESS_MODE_PERCENT}>Percent</option>
        </Select>

        {progressMode === PROGRESS_MODE_PAGES ? (
          <>
            <Input
              type="number"
              min={0}
              value={currentPage}
              onChange={(e) => setCurrentPage(Number(e.target.value))}
              onFocus={(e) => e.target.select()}
              onBlur={() => void handleCommit()}
              autoFocus
              aria-label="Current page"
              className="w-20"
            />
            {userBook.book?.pageCount ? (
              <span className="text-xs text-muted whitespace-nowrap">
                / {userBook.book.pageCount}
              </span>
            ) : null}
          </>
        ) : (
          <>
            <Input
              type="number"
              min={0}
              max={100}
              value={progressPercent}
              onChange={(e) => setProgressPercent(Number(e.target.value))}
              onFocus={(e) => e.target.select()}
              onBlur={() => void handleCommit()}
              autoFocus
              aria-label="Progress percent"
              className="w-20"
            />
            <span className="text-xs text-muted">%</span>
          </>
        )}
      </div>
      <p className="text-xs text-muted">Press Enter to save, Escape to cancel</p>
    </div>
  )
}
