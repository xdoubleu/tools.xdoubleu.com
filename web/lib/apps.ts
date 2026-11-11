import {
  Activity,
  Bot,
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
    href: '/dashboard/reading',
    description: 'Search, library and reading progress.',
    base: '/dashboard/reading',
    group: 'Media',
    icon: BookOpen,
    artwork: true
  },
  {
    id: 'games',
    label: 'Games',
    href: '/dashboard/games',
    description: 'Steam backlog, progress and distribution.',
    base: '/dashboard/games',
    group: 'Media',
    icon: Gamepad2,
    artwork: true
  },
  {
    id: 'movies',
    label: 'Movies & series',
    href: '/movies',
    description: 'Watch backlog with TMDB search.',
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
    group: 'Admin',
    icon: Factory,
    adminOnly: true
  },
  {
    id: 'monitoring',
    label: 'Monitoring',
    href: '/monitoring/connections',
    description: 'Observability',
    base: '/monitoring',
    group: 'Admin',
    icon: Activity,
    adminOnly: true
  },
  {
    id: 'observability',
    label: 'Automated actions',
    href: '/monitoring/observability',
    description: 'Self-healing routine run history',
    base: '/monitoring/observability',
    group: 'Admin',
    icon: Bot,
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

const baseOf = (app: AppEntry) => app.base ?? `/${app.href.split('/')[1]}`

/** The app that owns `pathname` (longest matching route prefix), if any. */
export function appForPath(pathname: string): AppEntry | undefined {
  const matches = APPS.filter((app) => {
    const base = baseOf(app)
    return pathname === base || pathname.startsWith(`${base}/`)
  })
  return matches.sort((a, b) => baseOf(b).length - baseOf(a).length)[0]
}
