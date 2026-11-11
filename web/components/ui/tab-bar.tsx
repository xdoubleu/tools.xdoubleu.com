import { type LucideIcon } from 'lucide-react'
import { Link } from '@/components/ui/link'
import { cn } from '@/lib/cn'

interface TabBarItem {
  id: string
  label: string
  href: string
  icon: LucideIcon
  active: boolean
}

interface TabBarProps {
  items: TabBarItem[]
  /** Names the landmark for screen readers. */
  'aria-label': string
  className?: string
}

// Stryker disable StringLiteral: class names are presentation; the active state is tested.
/**
 * The global phone bar: equal slots, each an icon over a short label, fixed to the
 * bottom edge above the home indicator. Hidden from `lg`, where the sidebar takes over.
 * The shell reserves `--tab-bar-height` below the page so nothing sits underneath it.
 */
function TabBar({ items, 'aria-label': ariaLabel, className }: TabBarProps) {
  return (
    <nav
      aria-label={ariaLabel}
      className={cn(
        'fixed inset-x-0 bottom-0 z-40 border-t border-border/60 bg-glass pb-[env(safe-area-inset-bottom)] pl-[env(safe-area-inset-left)] pr-[env(safe-area-inset-right)] backdrop-blur-xl backdrop-saturate-150 lg:hidden',
        className
      )}
    >
      <ul className="flex h-[var(--tab-bar-height)]">
        {items.map(({ id, label, href, icon: Icon, active }) => (
          <li key={id} className="min-w-0 flex-1">
            <Link
              href={href}
              aria-current={active ? 'page' : undefined}
              className={cn(
                'flex h-full min-h-11 flex-col items-center justify-center gap-0.5 px-1 text-xs transition-colors',
                'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-accent',
                active ? 'font-medium text-accent' : 'text-muted active:text-fg'
              )}
            >
              <Icon aria-hidden className="size-6 shrink-0" />
              <span className="max-w-full truncate">{label}</span>
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  )
}
// Stryker restore StringLiteral

export { TabBar }
export type { TabBarItem, TabBarProps }
