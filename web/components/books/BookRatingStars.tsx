'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useUpdateBookStatus } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { cn } from '@/lib/cn'
import { swrKeys } from '@/lib/swrKeys'

interface BookRatingStarsProps {
  userBook: UserBook
  /** Render as a read-only display (no click handlers). */
  readOnly?: boolean
  /** "sm" = 14px stars (card); "md" = 18px stars (detail page). Default "sm". */
  size?: 'sm' | 'md'
  onSaved?: () => void
}

export default function BookRatingStars({
  userBook,
  readOnly = false,
  size = 'sm',
  onSaved
}: BookRatingStarsProps) {
  const [rating, setRating] = useState(userBook.rating)
  const [hover, setHover] = useState(0)
  const updateBookStatus = useUpdateBookStatus()

  const handleClick = async (star: number) => {
    if (readOnly) return
    // Clicking the current rating clears it (toggle off)
    const newRating = star === rating ? 0 : star
    const prev = rating
    setRating(newRating)
    try {
      await updateBookStatus({
        bookId: userBook.id,
        status: userBook.status,
        favourite: userBook.tags.includes('favourite'),
        rating: String(newRating)
      })
      mutate(swrKeys.books)
      onSaved?.()
    } catch {
      setRating(prev)
    }
  }

  const displayed = hover > 0 ? hover : rating

  return (
    <div
      className="flex items-center gap-0.5"
      aria-label={rating > 0 ? `${rating} out of 5 stars` : 'No rating'}
      onMouseLeave={() => setHover(0)}
    >
      {[1, 2, 3, 4, 5].map((star) => (
        <button
          key={star}
          type="button"
          onClick={() => handleClick(star)}
          onMouseEnter={() => !readOnly && setHover(star)}
          disabled={readOnly}
          aria-label={`Rate ${star} star${star > 1 ? 's' : ''}`}
          className={cn(
            'leading-none transition-colors',
            size === 'md' ? 'text-lg' : 'text-sm',
            readOnly ? 'cursor-default' : 'cursor-pointer',
            star <= displayed ? 'text-amber-400' : 'text-border',
            !readOnly &&
              'hover:text-amber-400 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent'
          )}
        >
          ★
        </button>
      ))}
    </div>
  )
}
