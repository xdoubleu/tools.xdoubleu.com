'use client'

import { useState } from 'react'
import Image from 'next/image'
import { cn } from '@/lib/cn'
import { posterUrl } from '@/lib/movies/format'

const SIZES = {
  sm: { width: 48, height: 72, tmdb: 'w92' },
  lg: { width: 128, height: 192, tmdb: 'w342' }
} as const

interface MoviePosterProps {
  posterPath: string
  title: string
  size?: keyof typeof SIZES
}

/** TMDB poster, hotlinked; a muted initial when there is none or it fails. */
export default function MoviePoster({ posterPath, title, size = 'sm' }: MoviePosterProps) {
  const [errored, setErrored] = useState(false)
  const { width, height, tmdb } = SIZES[size]
  const src = posterUrl(posterPath, tmdb)

  return (
    <div
      style={{ width, height, minWidth: width }}
      className="relative shrink-0 overflow-hidden rounded-lg"
    >
      {!src || errored ? (
        <div
          className={cn(
            'flex h-full w-full select-none items-center justify-center rounded-lg bg-surface',
            'font-semibold text-muted',
            size === 'lg' ? 'text-2xl' : 'text-sm'
          )}
          aria-hidden="true"
        >
          {title.charAt(0).toUpperCase()}
        </div>
      ) : (
        <Image
          src={src}
          alt=""
          width={width}
          height={height}
          className="h-full w-full rounded-lg object-cover"
          loading="lazy"
          onError={() => setErrored(true)}
        />
      )}
    </div>
  )
}
