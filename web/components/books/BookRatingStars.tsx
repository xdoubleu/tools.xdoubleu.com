'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useUpdateBookStatus } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { RatingStars } from '@/components/ui/rating-stars'
import { swrKeys } from '@/lib/swrKeys'

interface BookRatingStarsProps {
  userBook: UserBook
  /** Render as a read-only display (no click handlers). */
  readOnly?: boolean
  /** Glyph size: "sm" for cards, "md" for the detail page. */
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
  const updateBookStatus = useUpdateBookStatus()

  const handleChange = async (newRating: number) => {
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

  return (
    <RatingStars
      value={rating}
      size={size}
      onChange={readOnly ? undefined : (r) => void handleChange(r)}
    />
  )
}
