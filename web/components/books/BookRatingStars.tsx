'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useUpdateBookStatus } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { Button } from '@/components/ui/button'
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
        bookId: userBook.bookId,
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
        <Button
          key={star}
          variant="ghost"
          size="iconSm"
          onClick={() => handleClick(star)}
          onMouseEnter={() => !readOnly && setHover(star)}
          disabled={readOnly}
          aria-label={`Rate ${star} star${star > 1 ? 's' : ''}`}
          className={cn(
            'h-auto w-auto p-0 leading-none hover:bg-transparent',
            size === 'md' ? 'text-lg' : 'text-sm',
            star <= displayed ? 'text-amber-400' : 'text-border',
            // Read-only stars are a rating display, so they must stay legible
            // rather than taking the Button's dimmed disabled treatment.
            readOnly ? 'cursor-default disabled:opacity-100' : 'hover:text-amber-400'
          )}
        >
          ★
        </Button>
      ))}
    </div>
  )
}
