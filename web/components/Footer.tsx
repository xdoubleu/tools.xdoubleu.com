'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { getRelease } from '@/lib/env'
import BugReportModal from '@/components/BugReportModal'

export default function Footer() {
  const [release, setRelease] = useState<string>('')
  const [isBugReportOpen, setIsBugReportOpen] = useState(false)

  useEffect(() => {
    setRelease(getRelease())
  }, [])

  const year = new Date().getFullYear()

  return (
    <>
      <footer className="mt-auto border-t border-border bg-card px-4 py-3 text-xs sm:px-6">
        <div className="flex flex-wrap items-center justify-center gap-3 sm:gap-4 md:gap-6">
          <div className="text-muted">
            © {year}{' '}
            <Link href="https://xdoubleu.com" className="hover:text-fg">
              xdoubleu.com
            </Link>
          </div>

          {release && <div className="font-mono text-xs text-muted">{release.substring(0, 7)}</div>}

          <button
            onClick={() => setIsBugReportOpen(true)}
            className="text-muted hover:text-fg cursor-pointer"
          >
            Report a bug
          </button>
        </div>
      </footer>
      <BugReportModal isOpen={isBugReportOpen} onClose={() => setIsBugReportOpen(false)} />
    </>
  )
}
