'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'

const STARS = [1, 2, 3, 4, 5]

interface RatingStarsProps {
  /** 1–5 stars; 0 is unrated. */
  value: number
  /** Omit for a read-only display. Clicking the current rating passes 0 (clear). */
  onChange?: (value: number) => void
  disabled?: boolean
  /** Glyph size: "sm" for cards, "md" for detail pages. Interactive stars are 44px targets on phones. */
  size?: 'sm' | 'md'
}

/** Five-star rating; an image when read-only, star buttons otherwise. */
function RatingStars({ value, onChange, disabled = false, size = 'sm' }: RatingStarsProps) {
  const [hover, setHover] = useState(0)
  const label = value > 0 ? `${value} out of 5 stars` : 'No rating'

  if (!onChange) {
    return (
      <span
        role="img"
        aria-label={label}
        className={cn('inline-flex leading-none', size === 'md' ? 'text-lg' : 'text-sm')}
      >
        {STARS.map((star) => (
          <span key={star} aria-hidden className={star <= value ? 'text-star' : 'text-border'}>
            ★
          </span>
        ))}
      </span>
    )
  }

  const displayed = hover > 0 ? hover : value
  return (
    <div className="flex items-center" aria-label={label} onMouseLeave={() => setHover(0)}>
      {STARS.map((star) => (
        <Button
          key={star}
          variant="ghost"
          size="iconSm"
          disabled={disabled}
          onClick={() => onChange(star === value ? 0 : star)}
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

export { RatingStars }
export type { RatingStarsProps }
