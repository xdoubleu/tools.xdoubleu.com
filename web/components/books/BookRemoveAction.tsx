import { useState } from 'react'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { RemoveButton } from '@/components/ui/remove-button'
import RemoveBookDialog from '@/components/books/RemoveBookDialog'

interface BookRemoveActionProps {
  userBook: UserBook
  onSaved?: () => void
}

export default function BookRemoveAction({ userBook, onSaved }: BookRemoveActionProps) {
  const [open, setOpen] = useState(false)

  return (
    <>
      <RemoveButton
        aria-label={`Remove ${userBook.book?.title ?? 'book'} from library`}
        onClick={() => setOpen(true)}
      />
      <RemoveBookDialog
        userBook={userBook}
        title={userBook.book?.title ?? 'this book'}
        open={open}
        onOpenChange={setOpen}
        onRemoved={() => onSaved?.()}
      />
    </>
  )
}
