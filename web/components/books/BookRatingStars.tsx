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
  /** Glyph size: "sm" for cards, "md" for the detail page. Interactive stars are 44px targets on phones. */
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
    // Clicking the current rating clears it.
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
  const label = rating > 0 ? `${rating} out of 5 stars` : 'No rating'

  if (readOnly) {
    return (
      <span
        role="img"
        aria-label={label}
        className={cn('inline-flex leading-none', size === 'md' ? 'text-lg' : 'text-sm')}
      >
        {[1, 2, 3, 4, 5].map((star) => (
          <span key={star} aria-hidden className={star <= rating ? 'text-star' : 'text-border'}>
            ★
          </span>
        ))}
      </span>
    )
  }

  return (
    <div className="flex items-center" aria-label={label} onMouseLeave={() => setHover(0)}>
      {[1, 2, 3, 4, 5].map((star) => (
        <Button
          key={star}
          variant="ghost"
          size="iconSm"
          onClick={() => handleClick(star)}
          onMouseEnter={() => setHover(star)}
          aria-label={`Rate ${star} star${star > 1 ? 's' : ''}`}
          className={cn(
            'leading-none hover:bg-transparent hover:text-star',
            size === 'md' && 'text-xl sm:text-lg',
            star <= displayed ? 'text-star' : 'text-border'
          )}
        >
          ★
        </Button>
      ))}
    </div>
  )
}
