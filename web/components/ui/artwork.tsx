import { useState } from 'react'
import { cn } from '@/lib/cn'

// The fallback's text size per box, for one initial and for two.
const SIZES = {
  sm: { width: 40, height: 60, text: ['text-xs', 'text-[10px]'] },
  md: { width: 48, height: 72, text: ['text-sm', 'text-xs'] },
  lg: { width: 128, height: 192, text: ['text-2xl', 'text-base'] },
  square: { width: 64, height: 64, text: ['text-sm', 'text-xs'] },
  wide: { width: 112, height: 64, text: ['text-sm', 'text-xs'] }
} as const

interface ArtworkProps {
  /** Hotlinked image; the fallback shows when empty or when it fails to load. */
  src: string
  /** What the artwork shows; the fallback is its initials. */
  title: string
  /** Tried once when `src` fails to load, before the initials. */
  fallbackSrc?: string
  /** sm/md/lg are 2:3 covers and posters; square is 64px; wide is 16:9. */
  size: keyof typeof SIZES
  /** Words the fallback takes an initial from. */
  initials?: 1 | 2
  /** Empty by default: a caption beside the artwork usually names it. */
  alt?: string
  className?: string
}

/** A cover, poster or artwork thumbnail that keeps its box, with a muted-initials fallback. */
export function Artwork({
  src,
  title,
  size,
  fallbackSrc,
  initials = 1,
  alt = '',
  className
}: ArtworkProps) {
  const [failed, setFailed] = useState(0)
  const shown = failed === 0 ? src : failed === 1 ? fallbackSrc : ''
  const { width, height, text } = SIZES[size]
  const letters = title
    .trim()
    .split(/\s+/)
    .slice(0, initials)
    .map((word) => word.charAt(0).toUpperCase())
    .join('')

  return (
    <div
      style={{ width, height, minWidth: width }}
      className={cn('relative shrink-0 overflow-hidden rounded-lg', className)}
    >
      {!shown ? (
        <div
          className={cn(
            'flex h-full w-full select-none items-center justify-center rounded-lg bg-surface font-semibold text-muted',
            text[initials - 1]
          )}
          aria-hidden="true"
        >
          {letters}
        </div>
      ) : (
        <img
          src={shown}
          alt={alt}
          width={width}
          height={height}
          className="h-full w-full rounded-lg object-cover"
          loading="lazy"
          onError={() => setFailed((n) => n + 1)}
        />
      )}
    </div>
  )
}
