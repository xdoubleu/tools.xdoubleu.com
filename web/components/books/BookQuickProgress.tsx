'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useUpdateProgress } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookProgressBar from '@/components/books/BookProgressBar'
import BookProgressForm from '@/components/books/BookProgressForm'
import { Button } from '@/components/ui/button'
import {
  PROGRESS_MODE_PAGES,
  PROGRESS_MODE_PERCENT,
  defaultProgressMode
} from '@/lib/books/bookProgress'
import { swrKeys } from '@/lib/swrKeys'

/** Step applied by one tap of the −/+ buttons, per progress mode. */
export const PAGE_STEP = 10
export const PERCENT_STEP = 5

/**
 * Reading progress with one-tap adjustment: the progress bar plus −/+ buttons
 * that commit a step immediately, and a tap on the bar to open the exact-entry
 * `BookProgressForm`. Built for the reading dashboard's currently-reading
 * cards, where updating progress is the most frequent action and a trip to the
 * detail page (especially on mobile) is the main cost.
 */
export default function BookQuickProgress({
  userBook,
  onSaved
}: {
  userBook: UserBook
  onSaved?: () => void
}) {
  const [editing, setEditing] = useState(false)
  // Local override so the bar moves on tap instead of waiting for the
  // revalidated library; cleared whenever the server value catches up.
  const [pending, setPending] = useState<{ page: number; percent: number } | null>(null)
  const [isSaving, setIsSaving] = useState(false)
  const updateProgress = useUpdateProgress()

  const mode = defaultProgressMode(userBook)
  const pageCount = userBook.book?.pageCount ?? 0
  // Pages mode without a known page count has nothing to step against — the
  // bar can't render a meaningful width either, so only offer exact entry.
  const canStep = mode === PROGRESS_MODE_PERCENT || pageCount > 0

  const current = pending ?? {
    page: userBook.currentPage,
    percent: userBook.progressPercent
  }
  const shown = { ...userBook, currentPage: current.page, progressPercent: current.percent }

  const step = async (direction: 1 | -1) => {
    if (isSaving) return
    const next =
      mode === PROGRESS_MODE_PAGES
        ? {
            page: clamp(current.page + direction * PAGE_STEP, pageCount),
            percent: current.percent
          }
        : {
            page: current.page,
            percent: clamp(current.percent + direction * PERCENT_STEP, 100)
          }
    if (next.page === current.page && next.percent === current.percent) return

    setPending(next)
    setIsSaving(true)
    try {
      await updateProgress({
        bookId: userBook.bookId,
        progressMode: mode,
        currentPage: next.page,
        progressPercent: next.percent
      })
      void mutate(swrKeys.books)
      onSaved?.()
    } catch {
      setPending(null)
    } finally {
      setIsSaving(false)
    }
  }

  if (editing) {
    return (
      <BookProgressForm
        userBook={userBook}
        onSaved={() => {
          setPending(null)
          onSaved?.()
        }}
        onClose={() => setEditing(false)}
      />
    )
  }

  return (
    <div className="flex items-end gap-2">
      <Button
        variant="ghost"
        onClick={() => setEditing(true)}
        aria-label="Edit reading progress"
        className="-my-2 h-auto min-w-0 flex-1 justify-start rounded-lg px-0 py-2 text-left hover:bg-transparent"
      >
        <BookProgressBar userBook={shown} />
      </Button>

      {canStep && (
        <div className="flex shrink-0 gap-1">
          <Button
            variant="secondary"
            size="icon"
            aria-label={stepLabel('Decrease', mode)}
            disabled={isSaving}
            onClick={() => void step(-1)}
          >
            −
          </Button>
          <Button
            variant="secondary"
            size="icon"
            aria-label={stepLabel('Increase', mode)}
            disabled={isSaving}
            onClick={() => void step(1)}
          >
            +
          </Button>
        </div>
      )}
    </div>
  )
}

function stepLabel(verb: string, mode: string): string {
  return mode === PROGRESS_MODE_PAGES
    ? `${verb} progress by ${PAGE_STEP} pages`
    : `${verb} progress by ${PERCENT_STEP} percent`
}

function clamp(value: number, max: number): number {
  if (value < 0) return 0
  if (value > max) return max
  return value
}
