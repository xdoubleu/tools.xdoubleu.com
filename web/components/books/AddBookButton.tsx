import { useState } from 'react'
import { mutate } from 'swr'
import { useSearchParams } from '@/hooks/navigation'
import AddManualBookDialog from '@/components/books/AddManualBookDialog'
import { Button } from '@/components/ui/button'
import { swrKeys } from '@/lib/swrKeys'

/** The library's create action: opens manual add, prefilled with the ?q= search. */
export default function AddBookButton() {
  const searchParams = useSearchParams()
  const [open, setOpen] = useState(false)

  return (
    <>
      <Button type="button" onClick={() => setOpen(true)}>
        Add book
      </Button>
      {open && (
        <AddManualBookDialog
          initialTitle={(searchParams.get('q') ?? '').trim()}
          onClose={() => setOpen(false)}
          onAdded={() => void mutate(swrKeys.books)}
        />
      )}
    </>
  )
}
