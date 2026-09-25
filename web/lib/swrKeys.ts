// SWR cache keys. Readers and mutate() calls must both use these: SWR
// serializes keys, so drift silently breaks revalidation.

export const swrKeys = {
  currentUser: '/auth/current-user',
  userManagementUsers: '/user-management/users',
  monitoringOAuthConnections: '/monitoring/oauth-connections',
  monitoringNotificationSettings: '/monitoring/notification-settings',
  monitoringAutomatedActions: '/monitoring/automated-actions',
  family: '/family',
  // Not under /api: kamal-proxy routes that to the Go service.
  webRelease: '/release',

  trainsFeedInfo: '/trains/feed-info',
  trainsStations: (query: string) => ['/trains/stations', query] as const,
  trainsJourneys: (
    originStopId: string,
    destinationStopId: string,
    time: string,
    arriveBy: boolean
  ) => ['/trains/journeys', originStopId, destinationStopId, time, arriveBy] as const,
  trainsJourneyDetail: (journeyId: string) => ['/trains/journey', journeyId] as const,
  trainsSavedCommutes: '/trains/saved-commutes',

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
  // Local-only key for polling the kobo-gateway's /status.
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
  // Singular so mutateFeedItems' '/feeds/items' prefix sweep keeps bodies.
  feedItem: (id: string) => `/feeds/item/${id}`,
  feedStats: '/feeds/stats',
  // Owner's reading-dashboard feeds widget.
  feedsSummary: '/feeds/summary',
  unhealthyFeeds: '/feeds/unhealthy',

  mealPlans: '/mealplans',
  mealPlan: (id: string, offset: number) => `/mealplans/${id}?offset=${offset}`,
  mealSuggestions: (planId: string, mealDate: string, mealSlot: string) =>
    `/mealplans/${planId}/suggest?d=${mealDate}&s=${mealSlot}`,

  recipes: '/recipes',
  recipe: (id: string, servings?: number) =>
    servings ? `/recipes/${id}?servings=${servings}` : `/recipes/${id}`,

  learningPaths: '/learningpaths',
  learningPath: (id: string) => `/learningpaths/${id}`,
  todoistConnection: '/learningpaths/todoist-connection',

  shoppingList: (ownerUserId: string) => `/shoppinglist?owner=${ownerUserId}`,
  shoppingListExport: (planId: string, excludedGroups: readonly string[]) =>
    `/shoppinglist/export/${planId}?excluded=${[...excludedGroups].sort().join(',')}`,
  shoppingListExportAll: (excludedGroups: readonly string[]) =>
    `/shoppinglist/export/all?excluded=${[...excludedGroups].sort().join(',')}`,
  planIngredientGroups: (planId: string) => `/shoppinglist/groups/${planId}`,
  planIngredientGroupsAll: '/shoppinglist/groups/all',
  shoppingCategories: (ownerUserId: string) => `/shoppinglist/categories?owner=${ownerUserId}`,
  stores: '/shoppinglist/stores',
  storeCategories: (storeId: string) => `/shoppinglist/stores/${storeId}/categories`,
  itemNames: '/shoppinglist/item-names',
  itemCategories: '/shoppinglist/item-categories'
} as const
