'use client'

import Link from 'next/link'
import { useCurrentUser, useSignOut } from '@/hooks/useAuth'

export default function Navbar() {
  const { data, isLoading } = useCurrentUser()
  const signOut = useSignOut()

  if (isLoading || !data) return null

  const handleSignOut = async () => {
    await signOut()
    if (typeof window !== 'undefined') {
      window.location.href = '/'
    }
  }

  return (
    <header className="sticky top-0 z-50 border-b border-border/60 bg-glass backdrop-blur-xl backdrop-saturate-150 shadow-glass">
      <nav className="mx-auto flex max-w-7xl items-center justify-between px-4 py-2 sm:px-6">
        <Link
          href="/"
          className="text-sm font-semibold text-fg transition-colors hover:text-accent"
        >
          tools.xdoubleu.com
        </Link>
        <div className="flex items-center gap-1">
          <Link
            href="/settings"
            className="inline-flex min-h-9 items-center rounded-xl px-3 py-1.5 text-sm text-muted transition-colors hover:text-accent"
          >
            Settings
          </Link>
          <button
            onClick={handleSignOut}
            className="inline-flex min-h-9 items-center rounded-xl px-3 py-1.5 text-sm text-muted transition-colors hover:text-accent"
          >
            Sign out
          </button>
        </div>
      </nav>
    </header>
  )
}
