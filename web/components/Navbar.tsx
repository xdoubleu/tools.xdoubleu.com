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
    <header className="border-b border-border bg-card px-6 py-3">
      <nav className="flex items-center justify-between">
        <Link href="/" className="text-sm font-semibold text-fg hover:text-blue-600">
          tools.xdoubleu.com
        </Link>
        <div className="flex items-center gap-4">
          <Link href="/settings" className="text-sm text-muted hover:text-blue-600">
            Settings
          </Link>
          <Link href="/contacts" className="text-sm text-muted hover:text-blue-600">
            Contacts
          </Link>
          <Link href="/admin" className="text-sm text-muted hover:text-blue-600">
            Admin
          </Link>
          <button onClick={handleSignOut} className="text-sm text-muted hover:text-blue-600">
            Sign out
          </button>
        </div>
      </nav>
    </header>
  )
}
