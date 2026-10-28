const fs = require('node:fs')
const path = require('node:path')

// Folders under app/, components/ and lib/ that serve every domain. lib/oauth2as
// belongs to app/oauth (it mirrors the api's package name).
const SHARED_FOLDERS = ['ui', 'notifications', 'gen', 'oauth2as', 'offline']

// hooks/ is flat, so each hook is assigned here; a hook in no domain is shared.
const DOMAIN_HOOKS = {
  books: [
    'useBooks',
    'useBookSeries',
    'useKoboGateway',
    'useOfflineBooks',
    'usePhysicalCopy',
    'useReadingState'
  ],
  dashboard: ['useDashboardChartState', 'useDashboardShare'],
  family: ['useFamily'],
  feeds: ['useFeeds', 'useFeedFilteredItems', 'useFeedRuleSuggestions'],
  games: ['useGames'],
  learningpaths: ['useLearningPaths', 'useTodoistConnection'],
  mealplans: ['useMealPlans'],
  monitoring: ['useMonitoring'],
  movies: ['useMovies'],
  podcasts: ['usePodcasts'],
  recipes: ['useRecipes'],
  shoppinglist: ['useShoppingList'],
  taste: ['useTaste'],
  trains: ['useTrains', 'useJourneySocket'],
  'user-management': ['useUserManagement'],
  watchparty: ['useWatchPartyRTC', 'useWatchPartyRooms']
}
const SHARED_HOOKS = [
  'navigation',
  'useAsyncAction',
  'useAuth',
  'useIngredientCatalog',
  'useKeyboardInset',
  'useLocalStorage',
  'useNotificationSettings',
  'usePaginatedList',
  'useProgressSocket',
  'useSharedList',
  'useWakeLock'
]

// The only cross-domain edges, importer → imported. Each follows the api:
// shoppinglist reads mealplans, which reads recipes; learningpaths resolves
// books and feeds; dashboard reaches every app (ADR-0007). Shared code
// (outside any domain) may import every domain.
const ALLOWED_EDGES = {
  learningpaths: ['books', 'feeds'],
  mealplans: ['recipes'],
  shoppinglist: ['mealplans', 'recipes']
}
const UNRESTRICTED_IMPORTERS = ['dashboard']

const subfolders = (dir) =>
  fs
    .readdirSync(path.join(__dirname, dir), { withFileTypes: true })
    .filter((entry) => entry.isDirectory() && !SHARED_FOLDERS.includes(entry.name))
    .map((entry) => entry.name)

const domains = [
  ...new Set([...subfolders('app'), ...subfolders('components'), ...subfolders('lib')])
]

const classified = new Set([...Object.values(DOMAIN_HOOKS).flat(), ...SHARED_HOOKS])
const unclassified = fs
  .readdirSync(path.join(__dirname, 'hooks'))
  .map((file) => file.replace(/\.tsx?$/, ''))
  .filter((hook) => !classified.has(hook))
if (unclassified.length > 0) {
  throw new Error(
    `.dependency-cruiser.cjs: add ${unclassified.join(', ')} to DOMAIN_HOOKS or SHARED_HOOKS`
  )
}

const domainPath = (domain) => {
  const hooks = DOMAIN_HOOKS[domain] ?? []
  const folders = `^(?:app|components|lib)/${domain}/`
  return hooks.length > 0 ? `${folders}|^hooks/(?:${hooks.join('|')})\\.tsx?$` : folders
}

module.exports = {
  forbidden: domains.map((domain) => ({
    name: `no-cross-domain-imports:${domain}`,
    comment: `Only ${domain} and its allowed importers may import ${domain}'s app/, components/, lib/ or hooks/ files.`,
    severity: 'error',
    from: {
      path: domains
        .filter(
          (importer) =>
            importer !== domain &&
            !UNRESTRICTED_IMPORTERS.includes(importer) &&
            !(ALLOWED_EDGES[importer] ?? []).includes(domain)
        )
        .map(domainPath)
    },
    to: { path: domainPath(domain) }
  })),
  options: {
    doNotFollow: {
      path: 'node_modules'
    },
    tsPreCompilationDeps: true,
    tsConfig: {
      fileName: 'tsconfig.json'
    },
    enhancedResolveOptions: {
      exportsFields: ['exports'],
      conditionNames: ['import', 'require', 'node', 'default']
    }
  }
}
