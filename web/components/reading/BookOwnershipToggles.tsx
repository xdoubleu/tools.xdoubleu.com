'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useToggleTag } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/reading/v1/library_pb'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { swrKeys } from '@/lib/swrKeys'
import TogglePill from '@/components/reading/TogglePill'

interface BookOwnershipTogglesProps {
  userBook: UserBook
  onSaved?: () => void
  /** Hide the "Ownership" label — used in the library table where the column header already says it. */
  hideLabel?: boolean
}

export default function BookOwnershipToggles({
  userBook,
  onSaved,
  hideLabel
}: BookOwnershipTogglesProps) {
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
    <div className="space-y-1.5">
      {!hideLabel && (
        <Label className="text-xs font-semibold text-muted uppercase tracking-wide">
          Ownership
        </Label>
      )}
      <div className="flex items-center gap-1.5 flex-wrap">
        <TogglePill
          label="Physical"
          active={ownPhysical}
          onClick={() => handleToggle(ownPhysical)}
        />
        {hasPdf && <Badge variant="secondary">PDF</Badge>}
        {hasEpub && <Badge variant="secondary">EPUB</Badge>}
      </div>
    </div>
  )
}
