'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useUpdateBookStatus } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import { cn } from '@/lib/cn'

interface BookFavouriteButtonProps {
  userBook: UserBook
  onSaved?: () => void
}

export default function BookFavouriteButton({ userBook, onSaved }: BookFavouriteButtonProps) {
  const [favourite, setFavourite] = useState(userBook.tags.includes('favourite'))
  const updateBookStatus = useUpdateBookStatus()

  const handleClick = async () => {
    const newFavourite = !favourite
    const prev = favourite
    setFavourite(newFavourite)
    try {
      await updateBookStatus({
        bookId: userBook.id,
        status: userBook.status,
        favourite: newFavourite,
        rating: String(userBook.rating)
      })
      mutate('/backlog/books')
      onSaved?.()
    } catch {
      setFavourite(prev)
    }
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      aria-label={favourite ? 'Remove from favourites' : 'Add to favourites'}
      aria-pressed={favourite}
      className={cn(
        'text-sm leading-none transition-colors',
        'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent',
        favourite ? 'text-amber-500' : 'text-border hover:text-amber-400'
      )}
    >
      ♥
    </button>
  )
}
