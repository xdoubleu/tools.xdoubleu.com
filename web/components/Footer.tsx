'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { getRelease, getApiUrl } from '@/lib/env'

export default function Footer() {
  const [webRelease, setWebRelease] = useState<string>('')
  const [apiRelease, setApiRelease] = useState<string>('')

  useEffect(() => {
    setWebRelease(getRelease())

    fetch(`${getApiUrl()}/api/version`)
      .then((res) => res.json())
      .then((data: { release?: string }) => setApiRelease(data.release ?? ''))
      .catch(() => {})
  }, [])

  const year = new Date().getFullYear()

  return (
    <footer className="border-t border-border/60 bg-glass backdrop-blur-xl backdrop-saturate-150 px-4 py-3 text-xs sm:px-6">
      <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-center gap-3 sm:gap-4">
        <div className="text-muted">
          © {year}{' '}
          <Link href="https://xdoubleu.com" className="underline hover:text-fg transition-colors">
            xdoubleu
          </Link>
        </div>

        {(webRelease || apiRelease) && (
          <div className="flex gap-2 font-mono text-xs text-muted">
            {webRelease && <span>{`web:${webRelease.substring(0, 7)}`}</span>}
            {apiRelease && <span>{`api:${apiRelease.substring(0, 7)}`}</span>}
          </div>
        )}
      </div>
    </footer>
  )
}
