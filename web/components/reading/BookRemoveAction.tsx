'use client'

import { useState } from 'react'
import type { UserBook } from '@/lib/gen/reading/v1/library_pb'
import { Button } from '@/components/ui/button'
import RemoveBookDialog from '@/components/reading/RemoveBookDialog'

interface BookRemoveActionProps {
  userBook: UserBook
  onSaved?: () => void
}

export default function BookRemoveAction({ userBook, onSaved }: BookRemoveActionProps) {
  const [open, setOpen] = useState(false)

  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="iconSm"
        className="text-danger hover:text-danger"
        aria-label={`Remove ${userBook.book?.title ?? 'book'} from library`}
        onClick={() => setOpen(true)}
      >
        ✕
      </Button>
      <RemoveBookDialog
        bookId={userBook.bookId}
        title={userBook.book?.title ?? 'this book'}
        open={open}
        onOpenChange={setOpen}
        onRemoved={() => onSaved?.()}
      />
    </>
  )
}
