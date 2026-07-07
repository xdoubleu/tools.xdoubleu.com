// Central registry of SWR cache keys.
//
// Query hooks (readers) and mutate() calls (invalidators) must both go
// through this registry: SWR serializes keys, so a reader and an invalidator
// that drift apart silently stop revalidating each other. Values must stay
// byte-identical to the literals they replaced.

export const swrKeys = {
  currentUser: '/auth/current-user',
  adminUsers: '/admin/users',
  adminJobStats: (windowDays: number) => ['/admin/job-stats', windowDays] as const,
  adminUsageStats: (windowDays: number) => ['/admin/usage-stats', windowDays] as const,
  adminStorageStats: '/admin/storage-stats',
  adminDatabaseStats: '/admin/database-stats',
  contacts: '/contacts',
  sharedMealPlans: '/sharing/mealplans',

  books: '/books',
  booksProgress: (dateStart?: string, dateEnd?: string) =>
    ['/books/progress', dateStart, dateEnd] as const,
  koboDevices: '/books/kobo/devices',
  bookDuplicates: '/books/duplicates',
  resyncProposals: '/books/resync-proposals',
  bookSources: (bookId: string) => ['/books/sources', bookId] as const,
  kepubStatus: (bookId: string) => ['/books/kepub-status', bookId] as const,
  bookFile: (bookId: string, format: string) => ['/books/file', bookId, format] as const,

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
