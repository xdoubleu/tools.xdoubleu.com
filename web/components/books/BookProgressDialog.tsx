'use client'

import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookProgressForm from '@/components/books/BookProgressForm'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'

interface BookProgressDialogProps {
  userBook: UserBook
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved?: () => void
}

/** The one reading-progress editor: a bottom sheet on phones, a dialog from `sm` up. */
export default function BookProgressDialog({
  userBook,
  open,
  onOpenChange,
  onSaved
}: BookProgressDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent side="sheet">
        <DialogHeader>
          <div className="min-w-0">
            <DialogTitle>Update progress</DialogTitle>
            <DialogDescription className="truncate">{userBook.book?.title}</DialogDescription>
          </div>
          <DialogClose />
        </DialogHeader>
        {open && (
          <BookProgressForm
            userBook={userBook}
            onSaved={onSaved}
            onClose={() => onOpenChange(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}
