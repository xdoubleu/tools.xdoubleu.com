'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { Button } from '@/components/ui/button'
import { getApiUrl, getRelease } from '@/lib/env'

// api and web build independently, so show both releases.
async function fetchRelease(url: string): Promise<string> {
  try {
    const res = await fetch(url)
    if (!res.ok) return ''
    const data: { release?: string } = await res.json()
    return data.release ?? ''
  } catch {
    return ''
  }
}

function ReleaseBadge({ label, release }: { label: string; release: string }) {
  if (!release) return null
  return (
    <span className="font-mono text-xs text-muted">
      {label} {release.substring(0, 7)}
    </span>
  )
}

export default function Footer() {
  const [webRelease, setWebRelease] = useState<string>('')
  const [apiRelease, setApiRelease] = useState<string>('')

  useEffect(() => {
    setWebRelease(getRelease())
    fetchRelease(`${getApiUrl()}/api/version`).then(setApiRelease)
  }, [])

  const year = new Date().getFullYear()

  return (
    <footer className="border-t border-border/60 bg-glass backdrop-blur-xl backdrop-saturate-150 px-4 pt-1 pb-[calc(0.25rem+env(safe-area-inset-bottom))] text-xs sm:px-6 sm:py-3 lg:px-10">
      <div className="mx-auto flex flex-wrap items-center justify-center gap-3 sm:gap-4">
        <div className="text-muted">
          © {year}{' '}
          <Button asChild variant="link" className="text-xs font-normal text-muted underline">
            <Link href="https://xdoubleu.com">xdoubleu</Link>
          </Button>
        </div>

        <ReleaseBadge label="web" release={webRelease} />
        <ReleaseBadge label="api" release={apiRelease} />
      </div>
    </footer>
  )
}
