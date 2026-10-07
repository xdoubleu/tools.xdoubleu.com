'use client'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'

interface WatchedWhenDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  pending: boolean
  /** unknownDate is true for "A while ago". */
  onChoose: (unknownDate: boolean) => void
}

/** Asks when a whole series was watched, so old binges don't date as today. */
export default function WatchedWhenDialog({
  open,
  onOpenChange,
  title,
  pending,
  onChoose
}: WatchedWhenDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent side="sheet">
        <DialogHeader>
          <DialogTitle>When did you watch {title}?</DialogTitle>
          <DialogClose />
        </DialogHeader>
        <DialogDescription>Every aired season is marked watched.</DialogDescription>
        <DialogFooter>
          <Button variant="secondary" disabled={pending} onClick={() => onChoose(true)}>
            A while ago
          </Button>
          <Button disabled={pending} onClick={() => onChoose(false)}>
            {pending ? 'Saving…' : 'Today'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
