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
      <nav className="mx-auto flex max-w-5xl items-center justify-between px-4 py-3 sm:px-6">
        <Link
          href="/"
          className="text-sm font-semibold text-fg transition-colors hover:text-accent"
        >
          tools.xdoubleu.com
        </Link>
        <button
          onClick={handleSignOut}
          className="min-h-11 rounded-lg px-3 text-sm text-muted transition-colors hover:text-accent"
        >
          Sign out
        </button>
      </nav>
    </header>
  )
}
