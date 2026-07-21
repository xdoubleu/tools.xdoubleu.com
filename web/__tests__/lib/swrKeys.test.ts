import { swrKeys } from '@/lib/swrKeys'

// These assertions pin every key to the exact literal it replaced. SWR
// serializes keys, so any drift here silently splits the cache between
// readers and invalidators — do not "clean up" these strings.
describe('swrKeys', () => {
  it('static keys match their historical literals', () => {
    expect(swrKeys.currentUser).toBe('/auth/current-user')
    expect(swrKeys.adminUsers).toBe('/admin/users')
    expect(swrKeys.contacts).toBe('/contacts')
    expect(swrKeys.webRelease).toBe('/release')
    expect(swrKeys.sharedMealPlans).toBe('/sharing/mealplans')
    expect(swrKeys.books).toBe('/reading')
    expect(swrKeys.koboDevices).toBe('/reading/kobo/devices')
    expect(swrKeys.bookDuplicates).toBe('/reading/duplicates')
    expect(swrKeys.resyncProposals).toBe('/reading/resync-proposals')
    expect(swrKeys.games).toBe('/games')
    expect(swrKeys.gamesRecent).toBe('/games/recent')
    expect(swrKeys.gamesIntegrations).toBe('/games/integrations')
    expect(swrKeys.icsFeeds).toBe('/icsproxy')
    expect(swrKeys.mealPlans).toBe('/mealplans')
    expect(swrKeys.recipes).toBe('/recipes')
    expect(swrKeys.recipeBookShares).toBe('/recipes/book-shares')
    expect(swrKeys.accessibleShoppingLists).toBe('/shoppinglist/accessible')
    expect(swrKeys.shoppingListShares).toBe('/shoppinglist/shares')
    expect(swrKeys.stores).toBe('/shoppinglist/stores')
    expect(swrKeys.itemNames).toBe('/shoppinglist/item-names')
    expect(swrKeys.itemCategories).toBe('/shoppinglist/item-categories')
    expect(swrKeys.planIngredientGroupsAll).toBe('/shoppinglist/groups/all')
    expect(swrKeys.todos).toBe('/todos')
    expect(swrKeys.todoSettings).toBe('/todos/settings')
  })

  it('dynamic keys match their historical shapes', () => {
    expect(swrKeys.booksProgress('a', 'b')).toEqual(['/reading/progress', 'a', 'b'])
    expect(swrKeys.kepubStatus('42')).toEqual(['/reading/kepub-status', '42'])
    expect(swrKeys.bookFile('42', 'epub')).toEqual(['/reading/file', '42', 'epub'])
    expect(swrKeys.game(7)).toBe('/games/7')
    expect(swrKeys.gamesDistribution(3)).toBe('/games/distribution/3')
    expect(swrKeys.gamesProgress('a', 'b')).toEqual(['/games/progress', 'a', 'b'])
    expect(swrKeys.icsPreview('https://x.test/a b')).toBe(
      '/icsproxy/preview?url=https%3A%2F%2Fx.test%2Fa%20b'
    )
    expect(swrKeys.icsConfig('tok')).toBe('/icsproxy/tok')
    expect(swrKeys.mealPlan('p1', 2)).toBe('/mealplans/p1?offset=2')
    expect(swrKeys.mealSuggestions('p1', '2026-07-04', 'dinner')).toBe(
      '/mealplans/p1/suggest?d=2026-07-04&s=dinner'
    )
    expect(swrKeys.profileBooksFeeds('tok')).toBe('/profile/reading/tok/feeds')
    expect(swrKeys.recipe('r1')).toBe('/recipes/r1')
    expect(swrKeys.recipe('r1', 4)).toBe('/recipes/r1?servings=4')
    expect(swrKeys.shoppingList('u1')).toBe('/shoppinglist?owner=u1')
    expect(swrKeys.shoppingListExport('p1', ['b', 'a'])).toBe(
      '/shoppinglist/export/p1?excluded=a,b'
    )
    expect(swrKeys.shoppingListExportAll(['b', 'a'])).toBe('/shoppinglist/export/all?excluded=a,b')
    expect(swrKeys.planIngredientGroups('p1')).toBe('/shoppinglist/groups/p1')
    expect(swrKeys.shoppingCategories('u1')).toBe('/shoppinglist/categories?owner=u1')
    expect(swrKeys.storeCategories('s1')).toBe('/shoppinglist/stores/s1/categories')
    expect(swrKeys.todosFiltered({ workspaceId: 'w' })).toEqual(['/todos', { workspaceId: 'w' }])
    expect(swrKeys.todoTask('t1')).toBe('/todos/tasks/t1')
  })

  it('does not mutate the excludedGroups argument', () => {
    const groups = ['b', 'a']
    swrKeys.shoppingListExport('p1', groups)
    expect(groups).toEqual(['b', 'a'])
  })

  it('webRelease stays off /api, which the DO ingress routes to the api service', () => {
    expect(swrKeys.webRelease.startsWith('/api')).toBe(false)
  })
})
