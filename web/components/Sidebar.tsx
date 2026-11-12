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
  'flex min-h-9 items-center gap-3 rounded-lg px-3 text-sm text-muted transition-colors hover:bg-hover hover:text-fg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent'
const collapsedItemClass = 'justify-center px-0'
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
    className: cn(itemClass, collapsed && collapsedItemClass, active && activeClass),
    children: (
      <>
        <app.icon aria-hidden className="size-5 shrink-0" />
        <span className={cn(collapsed && 'sr-only')}>{app.label}</span>
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
        'fixed inset-y-0 left-0 z-40 hidden flex-col border-r border-border/60 bg-glass pl-[var(--inset-left)] backdrop-blur-xl backdrop-saturate-150 lg:flex',
        collapsed ? 'w-16' : 'w-60'
      )}
    >
      <nav aria-label="Apps" className="flex-1 space-y-5 overflow-y-auto px-3 py-4">
        <Button
          variant="ghost"
          size="icon"
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          aria-expanded={!collapsed}
          onClick={onToggle}
          className={cn(
            'w-full text-muted hover:text-fg',
            collapsed ? 'justify-center' : 'justify-start px-3'
          )}
        >
          {collapsed ? (
            <PanelLeftOpen aria-hidden className="size-5 shrink-0" />
          ) : (
            <PanelLeftClose aria-hidden className="size-5 shrink-0" />
          )}
        </Button>
        <Link
          href="/"
          aria-current={pathname === '/' ? 'page' : undefined}
          className={cn(
            itemClass,
            collapsed && collapsedItemClass,
            pathname === '/' && activeClass
          )}
        >
          <House aria-hidden className="size-5 shrink-0" />
          <span className={cn(collapsed && 'sr-only')}>Today</span>
        </Link>
        {visibleSections(data)
          .filter((s) => s.title !== 'Account')
          .map((section) => (
            <section key={section.title}>
              <SectionHeading
                level="eyebrow"
                as="h2"
                className={cn('mb-1 px-3', collapsed && 'sr-only')}
              >
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
        <AccountMenu
          align="left"
          className={cn('w-full', collapsed ? 'justify-center' : 'justify-start')}
        />
      </div>
    </aside>
  )
}
// Stryker restore StringLiteral
