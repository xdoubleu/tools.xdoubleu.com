'use client'

import Link from 'next/link'
import { useCurrentUser, useSignOut } from '@/hooks/useAuth'
import { Button } from '@/components/ui/button'

const navItemClass = 'text-muted hover:bg-transparent hover:text-accent'

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
      <nav className="mx-auto flex items-center justify-between px-4 py-2 sm:px-6">
        <Link
          href="/"
          className="text-sm font-semibold text-fg transition-colors hover:text-accent"
        >
          tools.xdoubleu.com
        </Link>
        <div className="flex items-center gap-1">
          <Button asChild variant="ghost" size="sm" className={navItemClass}>
            <Link href="/settings">Settings</Link>
          </Button>
          <Button variant="ghost" size="sm" className={navItemClass} onClick={handleSignOut}>
            Sign out
          </Button>
        </div>
      </nav>
    </header>
  )
}
