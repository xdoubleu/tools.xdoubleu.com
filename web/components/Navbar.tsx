import { Link } from '@/components/ui/link'
import { usePathname } from '@/hooks/navigation'
import { useCurrentUser, useSignOut } from '@/hooks/useAuth'
import { Button } from '@/components/ui/button'
import SettingsIcon from '@/components/SettingsIcon'
import { flushOutbox } from '@/lib/offline/outbox'
import { clearOfflineData } from '@/lib/offline/session'
import { isSharedDashboard } from '@/lib/signIn'

const navItemClass = 'text-muted hover:bg-transparent hover:text-accent'
// Icon-only below `sm`; the text label stays for screen readers.
const labelClass = 'sr-only sm:not-sr-only'

function SignOutIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width={20}
      height={20}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className="sm:hidden"
    >
      <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
      <path d="m16 17 5-5-5-5" />
      <path d="M21 12H9" />
    </svg>
  )
}

export default function Navbar() {
  const { data, isLoading } = useCurrentUser()
  const signOut = useSignOut()
  const pathname = usePathname()

  if (isLoading || !data || isSharedDashboard(pathname)) return null

  const handleSignOut = async () => {
    // Send queued writes first; sign-out deletes whatever is left.
    await flushOutbox()
    await signOut()
    await clearOfflineData()
    if (typeof window !== 'undefined') {
      window.location.href = '/'
    }
  }

  return (
    <header className="sticky top-0 z-50 border-b border-border/60 bg-glass pt-[var(--inset-top)] backdrop-blur-xl backdrop-saturate-150 shadow-glass">
      <nav className="mx-auto flex items-center justify-between py-1 pl-[var(--inset-left)] pr-[max(0.5rem,env(safe-area-inset-right))] sm:py-2 sm:pr-[var(--inset-right)]">
        <Link
          href="/"
          className="flex min-h-11 min-w-0 items-center truncate text-sm font-semibold text-fg transition-colors hover:text-accent"
        >
          tools.xdoubleu.com
        </Link>
        <div className="flex items-center gap-2">
          <Button asChild variant="ghost" size="sm" className={navItemClass}>
            <Link href="/settings">
              <SettingsIcon size={20} className="sm:hidden" />
              <span className={labelClass}>Settings</span>
            </Link>
          </Button>
          <Button variant="ghost" size="sm" className={navItemClass} onClick={handleSignOut}>
            <SignOutIcon />
            <span className={labelClass}>Sign out</span>
          </Button>
        </div>
      </nav>
    </header>
  )
}
