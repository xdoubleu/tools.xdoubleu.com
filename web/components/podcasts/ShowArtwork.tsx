'use client'

import { useState } from 'react'
import Image from 'next/image'

const SIZE = 64

/** Show artwork, hotlinked from iTunes; a muted initial when none or it fails. */
export default function ShowArtwork({ url, title }: { url: string; title: string }) {
  const [errored, setErrored] = useState(false)

  return (
    <div
      style={{ width: SIZE, height: SIZE, minWidth: SIZE }}
      className="relative shrink-0 overflow-hidden rounded-lg"
    >
      {!url || errored ? (
        <div
          className="flex h-full w-full select-none items-center justify-center rounded-lg bg-surface text-sm font-semibold text-muted"
          aria-hidden="true"
        >
          {title.charAt(0).toUpperCase()}
        </div>
      ) : (
        <Image
          src={url}
          alt=""
          width={SIZE}
          height={SIZE}
          className="h-full w-full rounded-lg object-cover"
          loading="lazy"
          onError={() => setErrored(true)}
        />
      )}
    </div>
  )
}
