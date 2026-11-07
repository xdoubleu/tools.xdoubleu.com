import { type ReactNode } from 'react'
import { BottomTabs, type BottomTab } from '@/components/ui/bottom-tabs'
import { usePathname } from '@/hooks/navigation'
import { isSharedDashboard } from '@/lib/signIn'

function Icon({ children }: { children: ReactNode }) {
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
    >
      {children}
    </svg>
  )
}

const bookIcon = (
  <Icon>
    <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20V3H6.5A2.5 2.5 0 0 0 4 5.5z" />
  </Icon>
)
const chartIcon = (
  <Icon>
    <path d="M3 3v18h18" />
    <path d="M7 15v2M12 9v8M17 12v5" />
  </Icon>
)
const gamepadIcon = (
  <Icon>
    <rect x="2" y="7" width="20" height="10" rx="4" />
    <path d="M7 10v4M5 12h4M16 12h.01M18 12h.01" />
  </Icon>
)
const gearIcon = (
  <Icon>
    <circle cx="12" cy="12" r="3" />
    <path d="M12 2v3M12 19v3M2 12h3M19 12h3" />
  </Icon>
)

interface Section {
  label: string
  href: string
  icon: ReactNode
  /** The tab that owns every other page of the app. */
  fallback?: boolean
}

interface AppSections {
  name: string
  /** Paths that show the bar. */
  match: RegExp
  sections: Section[]
}

// Apps whose pages form real sections. Trains is one search flow with no
// sections, so it keeps the top bar alone.
const APPS: AppSections[] = [
  {
    name: 'Books',
    match: /^\/books\/(?!.*\/read$)/,
    sections: [
      { label: 'Library', href: '/books/library', icon: bookIcon, fallback: true },
      { label: 'Settings', href: '/books/settings', icon: gearIcon }
    ]
  },
  {
    name: 'Dashboard',
    match: /^\/dashboard\/(reading|games)\/?$/,
    sections: [
      { label: 'Reading', href: '/dashboard/reading', icon: chartIcon },
      { label: 'Games', href: '/dashboard/games', icon: gamepadIcon }
    ]
  }
]

const owns = (section: Section, pathname: string) =>
  pathname === section.href || pathname.startsWith(`${section.href}/`)

/** The phone-only bottom tab bar for the app the current page belongs to, if it has sections. */
export default function AppTabs() {
  const pathname = usePathname()
  if (isSharedDashboard(pathname)) return null
  const app = APPS.find((a) => a.match.test(pathname))
  if (!app) return null

  const exact = app.sections.find((s) => owns(s, pathname))
  const tabs: BottomTab[] = app.sections.map((s) => ({
    href: s.href,
    label: s.label,
    icon: s.icon,
    active: exact ? exact === s : s.fallback === true
  }))
  return <BottomTabs tabs={tabs} aria-label={`${app.name} sections`} />
}
