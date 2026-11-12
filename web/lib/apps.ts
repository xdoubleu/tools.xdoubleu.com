import {
  Activity,
  CalendarDays,
  ChartLine,
  ChefHat,
  Clapperboard,
  Factory,
  Gamepad2,
  GraduationCap,
  BookOpen,
  type LucideIcon,
  MonitorPlay,
  Podcast,
  Rss,
  Settings,
  ShieldCheck,
  ShoppingCart,
  Sparkles,
  TrainFront,
  Users
} from 'lucide-react'

export type AppGroup = (typeof GROUPS)[number]

export interface AppEntry {
  id: string
  label: string
  href: string
  description: string
  group: AppGroup
  icon: LucideIcon
  /** Shown to every signed-in user, whatever their grants. */
  alwaysVisible?: boolean
  adminOnly?: boolean
  /** Route prefix the app owns; defaults to the first segment of `href`. */
  base?: string
  /** Further route prefixes the app owns, e.g. its dashboard. */
  extraBases?: string[]
  /** Pages the app's sections switch between (`AppSections`). */
  sections?: { label: string; href: string }[]
  /** Has its own home-screen artwork (public/icons/<id>.svg and -apple.png). */
  artwork?: boolean
  /** Plain anchor instead of `<Link>`, for targets served by another container (e.g. `/grafana`). */
  external?: boolean
}

// Stryker disable StringLiteral: labels, copy and routes are presentation data
const GROUPS = ['Learning', 'Media', 'Transit', 'Food', 'Tools', 'Account', 'Admin'] as const

export const APPS: AppEntry[] = [
  {
    id: 'feeds',
    label: 'Feeds',
    href: '/feeds',
    description: 'Unread RSS and newsletter articles.',
    sections: [
      { label: 'Inbox', href: '/feeds' },
      { label: 'Stats', href: '/feeds/stats' }
    ],
    group: 'Learning',
    icon: Rss,
    artwork: true
  },
  {
    id: 'learningpaths',
    label: 'Learning paths',
    href: '/learningpaths/list',
    description: 'Self-directed curricula with modules, items and progress tracking',
    group: 'Learning',
    icon: GraduationCap
  },
  {
    id: 'books',
    label: 'Books',
    href: '/books/library',
    description: 'Search, library and reading progress.',
    extraBases: ['/dashboard/reading'],
    sections: [
      { label: 'Library', href: '/books/library' },
      { label: 'Stats', href: '/dashboard/reading' }
    ],
    group: 'Media',
    icon: BookOpen,
    artwork: true
  },
  {
    id: 'games',
    label: 'Games',
    href: '/games/library',
    description: 'Steam backlog, progress and distribution.',
    extraBases: ['/dashboard/games'],
    sections: [
      { label: 'Library', href: '/games/library' },
      { label: 'Stats', href: '/dashboard/games' }
    ],
    group: 'Media',
    icon: Gamepad2,
    artwork: true
  },
  {
    id: 'movies',
    label: 'Movies & series',
    href: '/movies',
    description: 'Watch backlog with TMDB search.',
    sections: [
      { label: 'Backlog', href: '/movies' },
      { label: 'Stats', href: '/movies/stats' }
    ],
    group: 'Media',
    icon: Clapperboard
  },
  {
    id: 'podcasts',
    label: 'Podcasts',
    href: '/podcasts',
    description: 'Favourite shows, found through iTunes.',
    group: 'Media',
    icon: Podcast
  },
  {
    id: 'taste',
    label: 'Taste',
    href: '/taste',
    description: 'What you read, play, watch and listen to, across every app.',
    group: 'Media',
    icon: Sparkles
  },
  {
    id: 'trains',
    label: 'Trains',
    href: '/trains',
    description: 'SNCB/NMBS station pickers and route overview.',
    group: 'Transit',
    icon: TrainFront
  },
  {
    id: 'recipes',
    label: 'Recipes',
    href: '/recipes/list',
    description: 'Recipe management',
    group: 'Food',
    icon: ChefHat,
    artwork: true
  },
  {
    id: 'mealplans',
    label: 'Meal plans',
    href: '/mealplans',
    description: 'Weekly meal planning',
    group: 'Food',
    icon: CalendarDays,
    artwork: true
  },
  {
    id: 'shoppinglist',
    label: 'Shopping list',
    href: '/shoppinglist',
    description: 'Generate shopping lists from meal plans',
    group: 'Food',
    icon: ShoppingCart,
    artwork: true
  },
  {
    id: 'watchparty',
    label: 'Watch party',
    href: '/watchparty',
    description: 'WebRTC screen sharing',
    group: 'Tools',
    icon: MonitorPlay,
    artwork: true
  },
  {
    id: 'settings',
    label: 'Settings',
    href: '/settings',
    description: 'User preferences',
    group: 'Account',
    icon: Settings,
    alwaysVisible: true
  },
  {
    id: 'family',
    label: 'Family',
    href: '/family',
    description: 'Share one recipe book, meal plan and shopping list together',
    group: 'Account',
    icon: Users,
    alwaysVisible: true
  },
  {
    id: 'user-management',
    label: 'User management',
    href: '/user-management',
    description: 'Administration',
    group: 'Admin',
    icon: ShieldCheck,
    adminOnly: true
  },
  {
    id: 'factory',
    label: 'Factory',
    href: '/factory',
    description: 'Task board for the software factory.',
    sections: [
      { label: 'Board', href: '/factory' },
      { label: 'Runner', href: '/factory/runners' },
      { label: 'Repos', href: '/factory/repos' }
    ],
    group: 'Admin',
    icon: Factory,
    adminOnly: true
  },
  {
    id: 'monitoring',
    label: 'Monitoring',
    href: '/monitoring/connections',
    description: 'Connections and self-healing routine runs',
    base: '/monitoring',
    sections: [
      { label: 'Connections', href: '/monitoring/connections' },
      { label: 'Automated actions', href: '/monitoring/observability' }
    ],
    group: 'Admin',
    icon: Activity,
    adminOnly: true
  },
  {
    id: 'grafana',
    label: 'Grafana',
    href: '/grafana',
    description: 'Metrics dashboards and alerting',
    group: 'Admin',
    icon: ChartLine,
    adminOnly: true,
    external: true
  }
]

// Stryker restore StringLiteral

interface AppViewer {
  role: string
  appAccess?: string[]
  /** Saved tab bar pins; only read when `tabPinsCustomised`. */
  tabPins?: string[]
  tabPinsCustomised?: boolean
  /** The apps opened most over the last 30 whole days, most first. */
  suggestedPins?: string[]
}

/** The apps `user` may open, in registry order. */
export function visibleApps(user: AppViewer): AppEntry[] {
  const isAdmin = user.role === 'admin'
  // Stryker disable next-line ArrayDeclaration: a filler grant matches no app
  const access = new Set(user.appAccess ?? [])
  return APPS.filter((app) => {
    if (app.alwaysVisible) return true
    if (app.adminOnly) return isAdmin
    return isAdmin || access.has(app.id)
  })
}

/** Visible apps bucketed by group, empty groups dropped. */
export function visibleSections(user: AppViewer): { title: AppGroup; apps: AppEntry[] }[] {
  const apps = visibleApps(user)
  return GROUPS.map((title) => ({ title, apps: apps.filter((a) => a.group === title) })).filter(
    (s) => s.apps.length > 0
  )
}

/** The app with home-screen artwork whose id is the route's first `segment`, if any. */
export function artworkApp(segment: string): AppEntry | undefined {
  return APPS.find((a) => a.artwork && a.id === segment)
}

const basesOf = (app: AppEntry) => [
  app.base ?? `/${app.href.split('/')[1]}`,
  // Stryker disable next-line ArrayDeclaration: an empty fallback adds no prefix
  ...(app.extraBases ?? [])
]

const matchLength = (app: AppEntry, pathname: string) =>
  Math.max(
    0,
    ...basesOf(app)
      .filter((base) => pathname === base || pathname.startsWith(`${base}/`))
      .map((base) => base.length)
  )

/** The app that owns `pathname` (longest matching route prefix), if any. */
export function appForPath(pathname: string): AppEntry | undefined {
  return APPS.map((app) => ({ app, length: matchLength(app, pathname) }))
    .filter((m) => m.length > 0)
    .sort((a, b) => b.length - a.length)[0]?.app
}

const DEFAULT_PINS = ['books', 'feeds', 'mealplans']

/**
 * The (up to three) apps in the phone tab bar for someone who never chose: the
 * ones they open most, then the fixed favourites, then the rest.
 */
export function defaultPins(user: AppViewer): AppEntry[] {
  const open = visibleApps(user).filter((a) => !a.alwaysVisible && !a.adminOnly)
  const byId = (ids: string[]) => ids.flatMap((id) => open.filter((a) => a.id === id))
  // Stryker disable next-line ArrayDeclaration: a filler id matches no app
  const used = byId([...new Set(user.suggestedPins ?? [])])
  const ranked = [...used, ...byId(DEFAULT_PINS), ...open]
  return [...new Set(ranked)].slice(0, 3)
}

/** What the phone tab bar pins can be: apps the user can open, minus the account entries. */
export function pinnableApps(user: AppViewer): AppEntry[] {
  return visibleApps(user).filter((a) => !a.alwaysVisible)
}

/**
 * The apps in the tab bar. A saved choice keeps its order; a pin the user can no
 * longer open is replaced by the next default so no slot goes dead.
 */
export function resolvePins(user: AppViewer): AppEntry[] {
  if (!user.tabPinsCustomised) return defaultPins(user)
  const open = pinnableApps(user)
  const wanted = [...new Set(user.tabPins ?? [])]
  const saved = wanted.flatMap((id) => open.filter((a) => a.id === id))
  const lost = wanted.length - saved.length
  const fill = defaultPins(user).filter((a) => !saved.includes(a))
  return [...saved, ...fill.slice(0, lost)]
}
