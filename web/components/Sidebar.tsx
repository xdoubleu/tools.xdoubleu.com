import { House, PanelLeftClose, PanelLeftOpen } from 'lucide-react'
import AccountMenu from '@/components/AccountMenu'
import { Button } from '@/components/ui/button'
import { Link } from '@/components/ui/link'
import { SectionHeading } from '@/components/ui/section-heading'
import { usePathname } from '@/hooks/navigation'
import { useCurrentUser } from '@/hooks/useAuth'
import { appForPath, visibleSections, type AppEntry } from '@/lib/apps'
import { cn } from '@/lib/cn'
import { isSharedDashboard } from '@/lib/signIn'

// Stryker disable StringLiteral: class names and menu copy are presentation; tests assert the labels.
const itemClass =
  'flex min-h-9 items-center gap-3 overflow-hidden whitespace-nowrap rounded-lg px-2.5 text-sm text-muted transition-colors hover:bg-hover hover:text-fg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent'
// Icons keep their position in both states; only labels fade, so nothing jumps on toggle.
const labelClass = 'truncate transition-opacity duration-200 motion-reduce:transition-none'
const activeClass = 'bg-accent/10 font-medium text-accent hover:bg-accent/10 hover:text-accent'

function NavLink({
  app,
  active,
  collapsed
}: {
  app: AppEntry
  active: boolean
  collapsed: boolean
}) {
  const props = {
    href: app.href,
    'aria-current': active ? ('page' as const) : undefined,
    className: cn(itemClass, active && activeClass),
    children: (
      <>
        <app.icon aria-hidden className="size-5 shrink-0" />
        <span className={cn(labelClass, collapsed && 'opacity-0')}>{app.label}</span>
      </>
    )
  }
  // Targets served by another container (e.g. /grafana) need a full page load.
  return app.external ? <a {...props} /> : <Link {...props} />
}

/** Global navigation from `lg` up: Today, every app the user can open, and the account menu. */
export default function Sidebar({
  collapsed,
  onToggle
}: {
  collapsed: boolean
  onToggle: () => void
}) {
  const { data, isLoading } = useCurrentUser()
  const pathname = usePathname()

  if (isLoading || !data || isSharedDashboard(pathname)) return null

  const current = appForPath(pathname)?.id

  return (
    <aside
      className={cn(
        'fixed inset-y-0 left-0 z-40 hidden flex-col overflow-x-hidden border-r border-border/60 bg-glass pl-[env(safe-area-inset-left)] backdrop-blur-xl backdrop-saturate-150 transition-[width] duration-200 ease-out motion-reduce:transition-none lg:flex',
        collapsed ? 'w-16' : 'w-60'
      )}
    >
      <div className="p-3 pb-0">
        <Button
          variant="ghost"
          size="icon"
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          aria-expanded={!collapsed}
          onClick={onToggle}
          className="text-muted hover:text-fg"
        >
          {collapsed ? (
            <PanelLeftOpen aria-hidden className="size-5 shrink-0" />
          ) : (
            <PanelLeftClose aria-hidden className="size-5 shrink-0" />
          )}
        </Button>
      </div>
      <nav aria-label="Apps" className="flex-1 overflow-y-auto overflow-x-hidden px-3 py-4">
        <Link
          href="/"
          aria-current={pathname === '/' ? 'page' : undefined}
          className={cn(itemClass, pathname === '/' && activeClass)}
        >
          <House aria-hidden className="size-5 shrink-0" />
          <span className={cn(labelClass, collapsed && 'opacity-0')}>Today</span>
        </Link>
        {visibleSections(data)
          .filter((s) => s.title !== 'Account')
          .map((section) => (
            <section key={section.title} className="mt-2 border-t border-border/60 pt-2">
              <SectionHeading level="eyebrow" as="h2" className="sr-only">
                {section.title}
              </SectionHeading>
              <ul className="space-y-0.5">
                {section.apps.map((app) => (
                  <li key={app.id}>
                    <NavLink app={app} active={current === app.id} collapsed={collapsed} />
                  </li>
                ))}
              </ul>
            </section>
          ))}
      </nav>
      <div className="border-t border-border/60 p-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))]">
        <AccountMenu align="left" />
      </div>
    </aside>
  )
}
// Stryker restore StringLiteral
