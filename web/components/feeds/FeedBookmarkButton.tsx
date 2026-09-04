'use client'

import { useState } from 'react'
import { ToggleIconButton } from '@/components/ui/toggle-icon-button'
import { useUpdateItem } from '@/hooks/useFeeds'

interface FeedBookmarkButtonProps {
  itemId: string
  bookmarked: boolean
}

// FeedBookmarkButton toggles an item's bookmarked flag directly (Item is
// self-contained — no library/book linkage to go through, unlike
// BookFavouriteButton).
export default function FeedBookmarkButton({ itemId, bookmarked }: FeedBookmarkButtonProps) {
  const [isBookmarked, setIsBookmarked] = useState(bookmarked)
  const updateItem = useUpdateItem()

  const handleClick = async () => {
    const next = !isBookmarked
    setIsBookmarked(next)
    try {
      await updateItem(itemId, { bookmarked: next })
    } catch {
      setIsBookmarked(!next)
    }
  }

  return (
    <ToggleIconButton
      active={isBookmarked}
      onToggle={handleClick}
      label="Bookmark"
      activeLabel="Remove bookmark"
    >
      <svg viewBox="0 0 16 16" className="h-4 w-4" fill="currentColor" aria-hidden="true">
        <path d="M3 1.5A1.5 1.5 0 0 1 4.5 0h7A1.5 1.5 0 0 1 13 1.5V16l-5-3-5 3V1.5Z" />
      </svg>
    </ToggleIconButton>
  )
}
