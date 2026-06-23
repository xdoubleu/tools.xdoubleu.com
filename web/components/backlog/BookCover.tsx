'use client'

import { useState } from 'react'
import Image from 'next/image'
import { cn } from '@/lib/cn'

const SIZES = {
  sm: { width: 40, height: 60, text: 'text-[10px]' },
  md: { width: 48, height: 72, text: 'text-xs' }
} as const

type Size = keyof typeof SIZES

interface BookCoverProps {
  coverUrl: string
  title: string
  size?: Size
  className?: string
}

function initials(title: string): string {
  return title
    .split(/\s+/)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? '')
    .join('')
}

export default function BookCover({ coverUrl, title, size = 'sm', className }: BookCoverProps) {
  const [errored, setErrored] = useState(false)
  const { width, height, text } = SIZES[size]

  const showPlaceholder = !coverUrl || errored

  return (
    <div
      style={{ width, height, minWidth: width }}
      className={cn('relative shrink-0 rounded-lg overflow-hidden', className)}
    >
      {showPlaceholder ? (
        <div
          className={cn(
            'flex h-full w-full items-center justify-center',
            'bg-surface rounded-lg',
            text,
            'font-semibold text-muted select-none'
          )}
          aria-hidden="true"
        >
          {initials(title)}
        </div>
      ) : (
        <Image
          src={coverUrl}
          alt={title}
          width={width}
          height={height}
          className="object-cover rounded-lg h-full w-full"
          loading="lazy"
          onError={() => setErrored(true)}
        />
      )}
    </div>
  )
}
