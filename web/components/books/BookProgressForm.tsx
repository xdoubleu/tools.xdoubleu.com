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
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { swrKeys } from '@/lib/swrKeys'

interface BookProgressFormProps {
  userBook: UserBook
  onSaved?: () => void
  /** Called after a successful save, and on Cancel/Escape — lets an embedding popover/toggle close itself. */
  onClose?: () => void
}

/**
 * The reading-progress edit form: a mode select (pages/percent) plus a
 * numeric input, committed via the Save button or Enter — never on blur, since
 * a mobile numeric keypad often has no key that fires a real Enter keydown,
 * and blurring into the Cancel button would otherwise save right before the
 * value is discarded. Shared between the card view's click-to-toggle usage
 * (`BookProgressEditor`) and the library table's "Progress" column popover
 * (`BookProgressCell`).
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

  const handleCancel = () => {
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
    <div className="space-y-2" onKeyDown={handleKeyDown}>
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
              inputMode="numeric"
              min={0}
              value={currentPage}
              onChange={(e) => setCurrentPage(Number(e.target.value))}
              onFocus={(e) => e.target.select()}
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
              inputMode="numeric"
              min={0}
              max={100}
              value={progressPercent}
              onChange={(e) => setProgressPercent(Number(e.target.value))}
              onFocus={(e) => e.target.select()}
              autoFocus
              aria-label="Progress percent"
              className="w-20"
            />
            <span className="text-xs text-muted">%</span>
          </>
        )}
      </div>
      <div className="flex gap-2">
        <Button onClick={() => void handleCommit()} disabled={isSaving}>
          {isSaving ? 'Saving…' : 'Save'}
        </Button>
        <Button variant="secondary" onClick={handleCancel} disabled={isSaving}>
          Cancel
        </Button>
      </div>
    </div>
  )
}
