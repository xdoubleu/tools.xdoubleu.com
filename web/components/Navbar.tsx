import AccountMenu from '@/components/AccountMenu'
import { Link } from '@/components/ui/link'
import { usePathname } from '@/hooks/navigation'
import { useCurrentUser } from '@/hooks/useAuth'
import { useScrolledTitle } from '@/hooks/useScrolledTitle'
import { appForPath } from '@/lib/apps'
import { isSharedDashboard } from '@/lib/signIn'

/** Slim top bar below `lg`: where you are (the page title once its heading scrolls away), and the account menu. The sidebar covers wider screens. */
export default function Navbar() {
  const { data, isLoading } = useCurrentUser()
  const pathname = usePathname()
  const scrolledTitle = useScrolledTitle(pathname)

  if (isLoading || !data || isSharedDashboard(pathname)) return null

  const title = scrolledTitle ?? appForPath(pathname)?.label ?? 'tools.xdoubleu.com'

  return (
    <header className="sticky top-0 z-50 border-b border-border/60 bg-glass pt-[var(--inset-top)] backdrop-blur-xl backdrop-saturate-150 shadow-glass lg:hidden">
      <nav className="mx-auto flex items-center justify-between py-1 pl-[var(--inset-left)] pr-[max(0.5rem,env(safe-area-inset-right))] sm:py-2 sm:pr-[var(--inset-right)]">
        <Link
          href="/"
          className="flex min-h-11 min-w-0 items-center truncate text-sm font-semibold text-fg transition-colors hover:text-accent"
        >
          {title}
        </Link>
        <AccountMenu />
      </nav>
    </header>
  )
}
