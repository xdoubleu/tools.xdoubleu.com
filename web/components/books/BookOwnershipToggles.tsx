'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useToggleTag } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/cn'
import { swrKeys } from '@/lib/swrKeys'

interface BookOwnershipTogglesProps {
  userBook: UserBook
  onSaved?: () => void
}

function ToggleChip({
  label,
  active,
  onClick
}: {
  label: string
  active: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className="focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent rounded-full"
    >
      <Badge
        variant={active ? 'default' : 'secondary'}
        className={cn(
          'cursor-pointer transition-opacity',
          !active && 'opacity-50 hover:opacity-80'
        )}
      >
        {label}
      </Badge>
    </button>
  )
}

export default function BookOwnershipToggles({ userBook, onSaved }: BookOwnershipTogglesProps) {
  const [ownPhysical, setOwnPhysical] = useState(userBook.tags.includes('own-physical'))
  const toggleTag = useToggleTag()

  const handleToggle = async (current: boolean) => {
    setOwnPhysical(!current)
    try {
      await toggleTag(userBook.bookId, 'own-physical')
      mutate(swrKeys.books)
      onSaved?.()
    } catch {
      setOwnPhysical(current)
    }
  }

  const hasPdf = userBook.formats.includes('pdf')
  const hasEpub = userBook.formats.includes('epub')

  return (
    <div className="flex items-center gap-1 flex-wrap mt-1">
      <ToggleChip label="Physical" active={ownPhysical} onClick={() => handleToggle(ownPhysical)} />
      {hasPdf && <Badge variant="default">PDF</Badge>}
      {hasEpub && <Badge variant="default">EPUB</Badge>}
    </div>
  )
}
