import { House, LayoutGrid } from 'lucide-react'
import { TabBar, type TabBarItem } from '@/components/ui/tab-bar'
import { usePathname } from '@/hooks/navigation'
import { useCurrentUser } from '@/hooks/useAuth'
import { appForPath, resolvePins } from '@/lib/apps'
import { hidesTabBar } from '@/lib/shell'

/** True when the phone tab bar is on screen, so the shell can leave room for it. */
export function useTabBarVisible(): boolean {
  const { data } = useCurrentUser()
  const pathname = usePathname()
  return data !== undefined && !hidesTabBar(pathname)
}

/** Today, the pinned apps and Apps, the same on every signed-in page. */
export default function PhoneTabBar() {
  const { data } = useCurrentUser()
  const pathname = usePathname()

  if (!data || hidesTabBar(pathname)) return null

  const pins = resolvePins(data)
  const current = appForPath(pathname)?.id
  const pinned = pins.some((p) => p.id === current)

  // Stryker disable StringLiteral: the slot ids are React keys only.
  const items: TabBarItem[] = [
    { id: 'today', label: 'Today', href: '/', icon: House, active: pathname === '/' },
    ...pins.map((app) => ({
      id: app.id,
      label: app.label,
      href: app.href,
      icon: app.icon,
      active: app.id === current
    })),
    {
      id: 'apps',
      label: 'Apps',
      href: '/apps',
      icon: LayoutGrid,
      active: pathname !== '/' && !pinned
    }
  ]
  // Stryker restore StringLiteral

  return <TabBar items={items} aria-label="Primary" />
}
