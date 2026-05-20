'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { getRelease, getApiUrl } from '@/lib/env'
import BugReportModal from '@/components/BugReportModal'

export default function Footer() {
  const [webRelease, setWebRelease] = useState<string>('')
  const [apiRelease, setApiRelease] = useState<string>('')
  const [isBugReportOpen, setIsBugReportOpen] = useState(false)

  useEffect(() => {
    setWebRelease(getRelease())

    fetch(`${getApiUrl()}/api/version`)
      .then((res) => res.json())
      .then((data: { release?: string }) => setApiRelease(data.release ?? ''))
      .catch(() => {})
  }, [])

  const year = new Date().getFullYear()

  return (
    <>
      <footer className="border-t border-border bg-card px-4 py-3 text-xs sm:px-6">
        <div className="flex flex-wrap items-center justify-center gap-3 sm:gap-4 md:gap-6">
          <div className="text-muted">
            © {year}{' '}
            <Link href="https://xdoubleu.com" className="underline hover:text-fg">
              xdoubleu
            </Link>
          </div>

          {(webRelease || apiRelease) && (
            <div className="flex gap-2 font-mono text-xs text-muted">
              {webRelease && <span>{`web:${webRelease.substring(0, 7)}`}</span>}
              {apiRelease && <span>{`api:${apiRelease.substring(0, 7)}`}</span>}
            </div>
          )}

          <button
            onClick={() => setIsBugReportOpen(true)}
            className="cursor-pointer text-muted underline hover:text-fg"
          >
            Report a bug
          </button>
        </div>
      </footer>
      <BugReportModal isOpen={isBugReportOpen} onClose={() => setIsBugReportOpen(false)} />
    </>
  )
}
