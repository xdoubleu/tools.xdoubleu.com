'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useUpdateProgress } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import {
  PROGRESS_MODE_PAGES,
  PROGRESS_MODE_PERCENT,
  defaultProgressMode
} from '@/lib/backlog/bookProgress'
import BookProgressBar from '@/components/backlog/BookProgressBar'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'

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
  const updateProgress = useUpdateProgress()

  const handleCommit = async () => {
    if (isSaving) return
    setIsSaving(true)
    try {
      await updateProgress({ bookId: userBook.id, progressMode, currentPage, progressPercent })
      mutate('/backlog/books')
      onSaved?.()
      setEditing(false)
    } catch {
      // keep editing open so the user can retry
    } finally {
      setIsSaving(false)
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
    return (
      <button
        type="button"
        onClick={() => setEditing(true)}
        aria-label="Edit reading progress"
        className="w-full text-left focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent rounded-lg"
      >
        <BookProgressBar userBook={userBook} />
      </button>
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
