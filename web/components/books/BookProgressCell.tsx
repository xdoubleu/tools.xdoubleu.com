'use client'

import { useState } from 'react'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookProgressBar from '@/components/books/BookProgressBar'
import BookProgressDialog from '@/components/books/BookProgressDialog'
import { Button } from '@/components/ui/button'

interface BookProgressCellProps {
  userBook: UserBook
  onSaved?: () => void
}

/** Library-table progress cell: the bar itself opens `BookProgressDialog`. */
export default function BookProgressCell({ userBook, onSaved }: BookProgressCellProps) {
  const [open, setOpen] = useState(false)
  if (userBook.status !== 'currently-reading') return null

  return (
    <>
      <Button
        variant="ghost"
        onClick={() => setOpen(true)}
        className="h-auto min-h-11 w-full min-w-32 justify-start px-1 text-left font-normal"
        aria-label={`Edit reading progress for ${userBook.book?.title ?? 'book'}`}
      >
        <span className="w-full">
          <BookProgressBar userBook={userBook} />
        </span>
      </Button>
      <BookProgressDialog
        userBook={userBook}
        open={open}
        onOpenChange={setOpen}
        onSaved={onSaved}
      />
    </>
  )
}
