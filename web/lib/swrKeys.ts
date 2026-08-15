// Central registry of SWR cache keys.
//
// Query hooks (readers) and mutate() calls (invalidators) must both go
// through this registry: SWR serializes keys, so a reader and an invalidator
// that drift apart silently stop revalidating each other. Values must stay
// byte-identical to the literals they replaced.

export const swrKeys = {
  currentUser: '/auth/current-user',
  userManagementUsers: '/user-management/users',
  monitoringJobStats: (windowDays: number) => ['/monitoring/job-stats', windowDays] as const,
  monitoringUsageStats: (windowDays: number) => ['/monitoring/usage-stats', windowDays] as const,
  monitoringStorageStats: '/monitoring/storage-stats',
  monitoringDatabaseStats: '/monitoring/database-stats',
  monitoringFailingPullRequests: '/monitoring/failing-pull-requests',
  monitoringSentryIssues: '/monitoring/sentry-issues',
  monitoringSlowTransactions: '/monitoring/slow-transactions',
  monitoringDeployStatus: '/monitoring/deploy-status',
  monitoringOAuthConnections: '/monitoring/oauth-connections',
  contacts: '/contacts',
  // Must stay off the /api prefix: the DO ingress routes /api/* to the Go
  // api service, so a path under /api never reaches this Next.js route.
  webRelease: '/release',
  sharedMealPlans: '/sharing/mealplans',

  dashboardShare: (kind: 'games' | 'reading') => `/dashboard/share/${kind}`,
  dashboardReading: (token: string) => `/dashboard/reading/${token}`,
  dashboardReadingProgress: (token: string, dateStart?: string, dateEnd?: string) =>
    ['/dashboard/reading-progress', token, dateStart, dateEnd] as const,
  dashboardFeedsSummary: (token: string) => `/dashboard/reading/${token}/feeds-summary`,
  dashboardGames: (token: string) => `/dashboard/games/${token}`,
  dashboardGamesProgress: (token: string, dateStart?: string, dateEnd?: string) =>
    ['/dashboard/games-progress', token, dateStart, dateEnd] as const,
  dashboardGame: (token: string, gameId: number) => `/dashboard/games/${token}/${gameId}`,
  dashboardRecentGames: (token: string) => `/dashboard/games/${token}/recent`,

  books: '/books',
  booksProgress: (dateStart?: string, dateEnd?: string) =>
    ['/books/progress', dateStart, dateEnd] as const,
  koboDevices: '/books/kobo/devices',
  // Local-only key (no server round-trip) for polling the kobo-gateway
  // helper's /status — see lib/books/gatewayClient.ts.
  gatewayStatus: '/books/kobo/gateway-status',
  koboDeviceLogs: (id: string) => ['/books/kobo/logs', id] as const,
  bookDuplicates: '/books/duplicates',
  resyncProposals: '/books/resync-proposals',
  bookSources: (bookId: string, overrideTitle = '', overrideAuthor = '') =>
    ['/books/sources', bookId, overrideTitle, overrideAuthor] as const,
  bookSourceStats: '/books/source-stats',
  bookBooksInExactSources: (sources: string[]) =>
    ['/books/exact-sources', [...sources].sort().join(',')] as const,
  kepubStatus: (bookId: string) => ['/books/kepub-status', bookId] as const,
  bookFile: (bookId: string, format: string) => ['/books/file', bookId, format] as const,
  bookContent: (bookId: string) => ['/books/content', bookId] as const,
  externalBook: (provider: string, providerId: string) =>
    ['/books/external', provider, providerId] as const,

  games: '/games',
  game: (gameId: number) => `/games/${gameId}`,
  gamesDistribution: (bucket: number) => `/games/distribution/${bucket}`,
  gamesProgress: (dateStart?: string, dateEnd?: string) =>
    ['/games/progress', dateStart, dateEnd] as const,
  gamesRecent: '/games/recent',
  gamesIntegrations: '/games/integrations',

  feeds: '/feeds',
  feedItems: (unreadOnly: boolean, feedId?: string, bookmarkedOnly?: boolean) =>
    `/feeds/items?unread=${unreadOnly}&feed=${feedId ?? ''}&bookmarked=${bookmarkedOnly ?? false}`,
  feedStats: '/feeds/stats',
  // Owner's own reading dashboard feeds widget — a handful of unread items
  // via the authenticated FeedService, separate from the public
  // dashboardFeedsSummary above.
  feedsSummary: '/feeds/summary',

  icsFeeds: '/icsproxy',
  icsPreview: (sourceUrl: string) => `/icsproxy/preview?url=${encodeURIComponent(sourceUrl)}`,
  icsConfig: (token: string) => `/icsproxy/${token}`,

  mealPlans: '/mealplans',
  mealPlan: (id: string, offset: number) => `/mealplans/${id}?offset=${offset}`,
  mealSuggestions: (planId: string, mealDate: string, mealSlot: string) =>
    `/mealplans/${planId}/suggest?d=${mealDate}&s=${mealSlot}`,

  recipes: '/recipes',
  recipe: (id: string, servings?: number) =>
    servings ? `/recipes/${id}?servings=${servings}` : `/recipes/${id}`,
  recipeBookShares: '/recipes/book-shares',

  shoppingList: (ownerUserId: string) => `/shoppinglist?owner=${ownerUserId}`,
  shoppingListExport: (planId: string, excludedGroups: readonly string[]) =>
    `/shoppinglist/export/${planId}?excluded=${[...excludedGroups].sort().join(',')}`,
  shoppingListExportAll: (excludedGroups: readonly string[]) =>
    `/shoppinglist/export/all?excluded=${[...excludedGroups].sort().join(',')}`,
  planIngredientGroups: (planId: string) => `/shoppinglist/groups/${planId}`,
  planIngredientGroupsAll: '/shoppinglist/groups/all',
  shoppingCategories: (ownerUserId: string) => `/shoppinglist/categories?owner=${ownerUserId}`,
  accessibleShoppingLists: '/shoppinglist/accessible',
  shoppingListShares: '/shoppinglist/shares',
  stores: '/shoppinglist/stores',
  storeCategories: (storeId: string) => `/shoppinglist/stores/${storeId}/categories`,
  itemNames: '/shoppinglist/item-names',
  itemCategories: '/shoppinglist/item-categories',

  todos: '/todos',
  todosFiltered: (queryParams: { workspaceId?: string; sectionId?: string; status?: string }) =>
    ['/todos', queryParams] as const,
  todoTask: (id: string) => `/todos/tasks/${id}`,
  todoSettings: '/todos/settings'
} as const
