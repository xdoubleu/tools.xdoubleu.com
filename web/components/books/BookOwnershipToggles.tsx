'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useSetBookTag } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { swrKeys } from '@/lib/swrKeys'
import { TogglePill } from '@/components/ui/toggle-pill'

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
  const [ownBol, setOwnBol] = useState(userBook.tags.includes('own-bol'))
  const setBookTag = useSetBookTag()

  const handleToggle = async (tag: string, current: boolean, setState: (v: boolean) => void) => {
    setState(!current)
    try {
      await setBookTag(userBook.bookId, tag, !current)
      mutate(swrKeys.books)
      onSaved?.()
    } catch {
      setState(current)
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
          onClick={() => handleToggle('own-physical', ownPhysical, setOwnPhysical)}
        />
        <TogglePill
          label="bol.com"
          active={ownBol}
          onClick={() => handleToggle('own-bol', ownBol, setOwnBol)}
        />
        {hasPdf && <Badge variant="default">PDF</Badge>}
        {hasEpub && <Badge variant="default">EPUB</Badge>}
      </div>
    </div>
  )
}
