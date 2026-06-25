'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useToggleTag } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/cn'

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
  const [ownDigital, setOwnDigital] = useState(userBook.tags.includes('own-digital'))
  const toggleTag = useToggleTag()

  const handleToggle = async (
    tag: 'own-physical' | 'own-digital',
    current: boolean,
    setter: (v: boolean) => void
  ) => {
    setter(!current)
    try {
      await toggleTag(userBook.id, tag)
      mutate('/backlog/books')
      onSaved?.()
    } catch {
      setter(current)
    }
  }

  const hasPdf = userBook.formats.includes('pdf')
  const hasEpub = userBook.formats.includes('epub')

  if (!ownPhysical && !ownDigital && !hasPdf && !hasEpub) {
    // Show muted chips so the user can still toggle on
    return (
      <div className="flex items-center gap-1 flex-wrap mt-1">
        <ToggleChip
          label="Physical"
          active={false}
          onClick={() => handleToggle('own-physical', false, setOwnPhysical)}
        />
        <ToggleChip
          label="Digital"
          active={false}
          onClick={() => handleToggle('own-digital', false, setOwnDigital)}
        />
      </div>
    )
  }

  return (
    <div className="flex items-center gap-1 flex-wrap mt-1">
      <ToggleChip
        label="Physical"
        active={ownPhysical}
        onClick={() => handleToggle('own-physical', ownPhysical, setOwnPhysical)}
      />
      <ToggleChip
        label="Digital"
        active={ownDigital}
        onClick={() => handleToggle('own-digital', ownDigital, setOwnDigital)}
      />
      {hasPdf && <Badge variant="default">PDF</Badge>}
      {hasEpub && <Badge variant="default">EPUB</Badge>}
    </div>
  )
}
