'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { getRelease } from '@/lib/env'

export default function Footer() {
  const [release, setRelease] = useState<string>('')

  useEffect(() => {
    setRelease(getRelease())
  }, [])

  const year = new Date().getFullYear()

  return (
    <footer className="border-t border-border/60 bg-glass backdrop-blur-xl backdrop-saturate-150 px-4 py-3 text-xs sm:px-6 lg:px-10">
      <div className="mx-auto flex flex-wrap items-center justify-center gap-3 sm:gap-4">
        <div className="text-muted">
          © {year}{' '}
          <Link href="https://xdoubleu.com" className="underline hover:text-fg transition-colors">
            xdoubleu
          </Link>
        </div>

        {release && <span className="font-mono text-xs text-muted">{release.substring(0, 7)}</span>}
      </div>
    </footer>
  )
}
