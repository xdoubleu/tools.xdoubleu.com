// Central registry of SWR cache keys.
//
// Query hooks (readers) and mutate() calls (invalidators) must both go
// through this registry: SWR serializes keys, so a reader and an invalidator
// that drift apart silently stop revalidating each other. Values must stay
// byte-identical to the literals they replaced.

export const swrKeys = {
  currentUser: '/auth/current-user',
  adminUsers: '/admin/users',
  monitoringJobStats: (windowDays: number) => ['/monitoring/job-stats', windowDays] as const,
  monitoringUsageStats: (windowDays: number) => ['/monitoring/usage-stats', windowDays] as const,
  monitoringStorageStats: '/monitoring/storage-stats',
  monitoringDatabaseStats: '/monitoring/database-stats',
  contacts: '/contacts',
  // Must stay off the /api prefix: the DO ingress routes /api/* to the Go
  // api service, so a path under /api never reaches this Next.js route.
  webRelease: '/release',
  sharedMealPlans: '/sharing/mealplans',

  profileShare: (app: 'reading' | 'games') => `/profile/share/${app}`,
  profileBooks: (token: string) => `/profile/reading/${token}`,
  profileBooksProgress: (token: string, dateStart?: string, dateEnd?: string) =>
    ['/profile/reading-progress', token, dateStart, dateEnd] as const,
  profileGames: (token: string) => `/profile/games/${token}`,
  profileGamesProgress: (token: string, dateStart?: string, dateEnd?: string) =>
    ['/profile/games-progress', token, dateStart, dateEnd] as const,
  profileGame: (token: string, gameId: number) => `/profile/games/${token}/${gameId}`,
  profileRecentGames: (token: string) => `/profile/games/${token}/recent`,

  books: '/reading',
  booksProgress: (dateStart?: string, dateEnd?: string) =>
    ['/reading/progress', dateStart, dateEnd] as const,
  koboDevices: '/reading/kobo/devices',
  // Local-only key (no server round-trip) for polling the kobo-gateway
  // helper's /status — see lib/reading/gatewayClient.ts.
  gatewayStatus: '/reading/kobo/gateway-status',
  koboDeviceLogs: (id: string) => ['/reading/kobo/logs', id] as const,
  bookFeeds: '/reading/feeds',
  bookDuplicates: '/reading/duplicates',
  resyncProposals: '/reading/resync-proposals',
  bookSources: (bookId: string, overrideTitle = '', overrideAuthor = '') =>
    ['/reading/sources', bookId, overrideTitle, overrideAuthor] as const,
  bookSourceStats: '/reading/source-stats',
  bookBooksInExactSources: (sources: string[]) =>
    ['/reading/exact-sources', [...sources].sort().join(',')] as const,
  kepubStatus: (bookId: string) => ['/reading/kepub-status', bookId] as const,
  bookFile: (bookId: string, format: string) => ['/reading/file', bookId, format] as const,
  externalBook: (provider: string, providerId: string) =>
    ['/reading/external', provider, providerId] as const,

  games: '/games',
  game: (gameId: number) => `/games/${gameId}`,
  gamesDistribution: (bucket: number) => `/games/distribution/${bucket}`,
  gamesProgress: (dateStart?: string, dateEnd?: string) =>
    ['/games/progress', dateStart, dateEnd] as const,
  gamesRecent: '/games/recent',
  gamesIntegrations: '/games/integrations',

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
