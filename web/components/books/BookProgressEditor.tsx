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
import BookProgressBar from '@/components/books/BookProgressBar'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { swrKeys } from '@/lib/swrKeys'

interface BookProgressEditorProps {
  userBook: UserBook
  onSaved?: () => void
}

export default function BookProgressEditor({ userBook, onSaved }: BookProgressEditorProps) {
  const [editing, setEditing] = useState(false)
  const [progressMode, setProgressMode] = useState(defaultProgressMode(userBook))
  const [currentPage, setCurrentPage] = useState(userBook.currentPage)
  const [progressPercent, setProgressPercent] = useState(userBook.progressPercent)
  const [isSaving, setIsSaving] = useState(false)
  const [isStepping, setIsStepping] = useState(false)
  const updateProgress = useUpdateProgress()
  const pageCount = userBook.book?.pageCount ?? 0

  const commit = async (page: number, percent: number) => {
    await updateProgress({
      bookId: userBook.bookId,
      progressMode,
      currentPage: page,
      progressPercent: percent
    })
    mutate(swrKeys.books)
    onSaved?.()
  }

  const handleCommit = async () => {
    if (isSaving) return
    setIsSaving(true)
    try {
      await commit(currentPage, progressPercent)
      setEditing(false)
    } catch {
      // keep editing open so the user can retry
    } finally {
      setIsSaving(false)
    }
  }

  // handleStep bumps progress by one page/percentage point in a single click,
  // without opening the full editor — the common case of nudging progress
  // forward doesn't need to reopen and retype a value that barely changed
  // (issue #1337: that round trip took too many clicks/taps).
  const handleStep = async (delta: number) => {
    if (isStepping || isSaving) return
    setIsStepping(true)
    try {
      if (progressMode === PROGRESS_MODE_PAGES) {
        const max = pageCount > 0 ? pageCount : Number.MAX_SAFE_INTEGER
        const nextPage = Math.min(max, Math.max(0, currentPage + delta))
        setCurrentPage(nextPage)
        await commit(nextPage, progressPercent)
      } else {
        const nextPercent = Math.min(100, Math.max(0, progressPercent + delta))
        setProgressPercent(nextPercent)
        await commit(currentPage, nextPercent)
      }
    } catch {
      // save failed — put the displayed value back to what's actually stored
      setCurrentPage(userBook.currentPage)
      setProgressPercent(userBook.progressPercent)
    } finally {
      setIsStepping(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      void handleCommit()
    } else if (e.key === 'Escape') {
      setEditing(false)
      // reset to stored values
      setProgressMode(defaultProgressMode(userBook))
      setCurrentPage(userBook.currentPage)
      setProgressPercent(userBook.progressPercent)
    }
  }

  if (!editing) {
    const atMax =
      progressMode === PROGRESS_MODE_PAGES
        ? pageCount > 0 && currentPage >= pageCount
        : progressPercent >= 100

    return (
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => setEditing(true)}
          aria-label="Edit reading progress"
          className="flex-1 min-w-0 text-left focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent rounded-lg"
        >
          <BookProgressBar userBook={userBook} />
        </button>
        <Button
          type="button"
          variant="secondary"
          size="iconSm"
          onClick={() => void handleStep(1)}
          disabled={isStepping || atMax}
          aria-label={
            progressMode === PROGRESS_MODE_PAGES
              ? 'Advance one page'
              : 'Advance progress by one percent'
          }
          className="shrink-0"
        >
          +1
        </Button>
      </div>
    )
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
