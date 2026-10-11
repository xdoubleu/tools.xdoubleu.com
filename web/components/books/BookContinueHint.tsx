import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import type { ContinueHint } from '@/lib/books/physicalCopy'

interface BookContinueHintProps {
  hint: ContinueHint
  onUse: (page: number) => void
}

/** Where the Kobo or web reader left off, as a page to find in the physical copy. */
export default function BookContinueHint({ hint, onUse }: BookContinueHintProps) {
  return (
    <Alert tone="info" className="space-y-2">
      <p className="font-semibold">Continue on page ~{hint.page}</p>
      {hint.excerpt && <p className="italic">“{hint.excerpt}”</p>}
      <Button variant="secondary" size="sm" onClick={() => onUse(hint.page)}>
        Use page {hint.page}
      </Button>
    </Alert>
  )
}
