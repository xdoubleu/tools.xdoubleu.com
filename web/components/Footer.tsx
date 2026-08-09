'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { getApiUrl, getRelease } from '@/lib/env'

// api/gateway builds are cacheable and only recompile when their own source
// changes (see build-api.yml/build-gateway.yml), so each component can
// legitimately be a different commit than this web build — show all three
// instead of one potentially-misleading badge.
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
  const [gatewayRelease, setGatewayRelease] = useState<string>('')

  useEffect(() => {
    setWebRelease(getRelease())
    fetchRelease(`${getApiUrl()}/api/version`).then(setApiRelease)
    fetchRelease('/gateway/version').then(setGatewayRelease)
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

        <ReleaseBadge label="web" release={webRelease} />
        <ReleaseBadge label="api" release={apiRelease} />
        <ReleaseBadge label="gateway" release={gatewayRelease} />
      </div>
    </footer>
  )
}
