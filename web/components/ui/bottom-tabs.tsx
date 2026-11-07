import { useEffect, useRef, type ReactNode } from 'react'
import { Link } from '@/components/ui/link'
import { cn } from '@/lib/cn'

interface BottomTab {
  href: string
  label: string
  icon: ReactNode
  active: boolean
}

interface BottomTabsProps {
  tabs: BottomTab[]
  /** Names the navigation landmark for screen readers. */
  'aria-label': string
}

/**
 * An app's section navigation, docked to the bottom of phones in a glass bar
 * above the safe area (hidden from `sm` up). The active tab is a filled pill.
 * It publishes its height as `--bottom-offset`, which `ThumbAction` docks
 * above, and leaves a spacer so the footer stays reachable.
 */
function BottomTabs({ tabs, 'aria-label': ariaLabel }: BottomTabsProps) {
  const bar = useRef<HTMLElement>(null)

  useEffect(() => {
    const el = bar.current
    if (!el) return
    const root = document.documentElement
    const publish = () => {
      root.style.setProperty('--bottom-offset', `${el.offsetHeight}px`)
      // The bar already clears the home indicator; docked content above it must not.
      root.style.setProperty('--bottom-safe', '0px')
    }
    publish()
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(publish)
    observer?.observe(el)
    return () => {
      observer?.disconnect()
      root.style.removeProperty('--bottom-offset')
      root.style.removeProperty('--bottom-safe')
    }
  }, [])

  return (
    <>
      <div aria-hidden className="h-[var(--bottom-offset,0px)] sm:hidden" />
      <nav
        ref={bar}
        aria-label={ariaLabel}
        className="fixed inset-x-0 bottom-0 z-40 border-t border-border/60 bg-glass pb-[env(safe-area-inset-bottom)] pl-[var(--inset-left)] pr-[var(--inset-right)] pt-1 shadow-glass backdrop-blur-xl backdrop-saturate-150 sm:hidden"
      >
        <ul className="flex gap-2">
          {tabs.map((tab) => (
            <li key={tab.href} className="min-w-0 flex-1">
              <Link
                href={tab.href}
                aria-current={tab.active ? 'page' : undefined}
                className={cn(
                  'flex min-h-11 flex-col items-center justify-center gap-0.5 rounded-xl px-2 py-1 text-xs font-medium',
                  'transition-[color,background-color,transform] duration-press ease-spring active:scale-[0.97] motion-reduce:ease-linear motion-reduce:active:scale-100',
                  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50',
                  tab.active ? 'bg-fg text-bg' : 'text-muted hover:text-fg'
                )}
              >
                {tab.icon}
                <span className="max-w-full truncate">{tab.label}</span>
              </Link>
            </li>
          ))}
        </ul>
      </nav>
    </>
  )
}

export { BottomTabs }
export type { BottomTab, BottomTabsProps }
