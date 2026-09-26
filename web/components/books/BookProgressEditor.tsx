'use client'

import { useState, type ReactNode } from 'react'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookProgressBar from '@/components/books/BookProgressBar'
import BookProgressDialog from '@/components/books/BookProgressDialog'
import { Button } from '@/components/ui/button'
import { track } from '@/lib/analytics'

interface BookProgressEditorProps {
  userBook: UserBook
  onSaved?: () => void
  /** Extra controls on the button row, e.g. "Mark as completed". */
  actions?: ReactNode
}

/** Progress bar with an explicit "Update progress" button that opens `BookProgressDialog`. */
export default function BookProgressEditor({
  userBook,
  onSaved,
  actions
}: BookProgressEditorProps) {
  const [open, setOpen] = useState(false)
  const title = userBook.book?.title

  return (
    <div className="w-full min-w-0 space-y-2">
      <BookProgressBar userBook={userBook} />
      <div className="flex flex-wrap gap-2">
        <Button
          variant="secondary"
          size="sm"
          onClick={() => {
            track('book_progress_dialog_opened', { source: 'progress_editor' })
            setOpen(true)
          }}
          aria-label={title ? `Update progress for ${title}` : 'Update progress'}
        >
          Update progress
        </Button>
        {actions}
      </div>
      <BookProgressDialog
        userBook={userBook}
        source="progress_editor"
        open={open}
        onOpenChange={setOpen}
        onSaved={onSaved}
      />
    </div>
  )
}
