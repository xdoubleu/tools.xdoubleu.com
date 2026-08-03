'use client'

import { useState } from 'react'
import { cn } from '@/lib/cn'
import { useUpdateItem } from '@/hooks/useFeeds'

interface FeedFavouriteButtonProps {
  itemId: string
  favourite: boolean
}

// FeedFavouriteButton toggles an item's favourite flag directly (Item is
// self-contained — no library/book linkage to go through, unlike
// BookFavouriteButton).
export default function FeedFavouriteButton({ itemId, favourite }: FeedFavouriteButtonProps) {
  const [isFavourite, setIsFavourite] = useState(favourite)
  const updateItem = useUpdateItem()

  const handleClick = async () => {
    const next = !isFavourite
    setIsFavourite(next)
    try {
      await updateItem(itemId, { favourite: next })
    } catch {
      setIsFavourite(!next)
    }
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      aria-label={isFavourite ? 'Remove from favourites' : 'Add to favourites'}
      aria-pressed={isFavourite}
      className={cn(
        'text-sm leading-none transition-colors',
        'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent',
        isFavourite ? 'text-amber-500' : 'text-border hover:text-amber-400 active:text-amber-400'
      )}
    >
      ♥
    </button>
  )
}
