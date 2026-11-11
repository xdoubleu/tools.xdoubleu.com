import {
  APPS,
  appForPath,
  artworkApp,
  defaultPins,
  pinnableApps,
  resolvePins,
  visibleApps,
  visibleSections
} from '@/lib/apps'

const ids = (apps: { id: string }[]) => apps.map((a) => a.id)

describe('visibleApps', () => {
  it('shows every app to an admin', () => {
    expect(visibleApps({ role: 'admin' })).toEqual(APPS)
  })

  it('shows a regular user only granted and always-visible apps', () => {
    const result = ids(visibleApps({ role: 'user', appAccess: ['books', 'feeds'] }))
    expect(result).toEqual(['feeds', 'books', 'settings', 'family'])
  })

  it('hides admin-only apps from users and treats missing grants as none', () => {
    const result = ids(visibleApps({ role: 'user' }))
    expect(result).toEqual(['settings', 'family'])
  })
})

describe('visibleSections', () => {
  it('groups in order and drops empty groups', () => {
    const sections = visibleSections({ role: 'user', appAccess: ['trains'] })
    expect(sections.map((s) => s.title)).toEqual(['Transit', 'Account'])
  })
})

describe('artworkApp', () => {
  it('finds apps with their own artwork only', () => {
    expect(artworkApp('recipes')?.id).toBe('recipes')
    expect(artworkApp('movies')).toBeUndefined()
    expect(artworkApp('')).toBeUndefined()
  })
})

describe('registry', () => {
  it('flags the apps with artwork and the external ones', () => {
    expect(ids(APPS.filter((a) => a.artwork))).toEqual([
      'feeds',
      'books',
      'games',
      'recipes',
      'mealplans',
      'shoppinglist',
      'watchparty'
    ])
    expect(ids(APPS.filter((a) => a.external))).toEqual(['grafana'])
  })

  it('has unique ids and an icon on every app', () => {
    expect(new Set(ids(APPS)).size).toBe(APPS.length)
    expect(APPS.every((a) => a.icon)).toBe(true)
  })
})

describe('appForPath', () => {
  it.each([
    ['/feeds', 'feeds'],
    ['/recipes/list', 'recipes'],
    ['/recipes/abc-123', 'recipes'],
    ['/dashboard/reading', 'books'],
    ['/dashboard/games/library', 'games'],
    ['/monitoring/observability/runs', 'observability'],
    ['/monitoring/connections', 'monitoring'],
    ['/monitoring/issues', 'monitoring'],
    ['/settings', 'settings']
  ])('maps %s to %s', (path, id) => {
    expect(appForPath(path)?.id).toBe(id)
  })

  it('does not match a longer sibling segment or the home page', () => {
    expect(appForPath('/feedsfoo')).toBeUndefined()
    expect(appForPath('/')).toBeUndefined()
    expect(appForPath('/dashboard')).toBeUndefined()
  })
})

describe('defaultPins with usage', () => {
  const user = {
    role: 'user',
    appAccess: ['books', 'feeds', 'mealplans', 'recipes', 'trains']
  }

  it('puts the most opened apps first, then the fixed favourites', () => {
    expect(ids(defaultPins({ ...user, suggestedPins: ['recipes', 'trains'] }))).toEqual([
      'recipes',
      'trains',
      'books'
    ])
  })

  it('takes the top three when there are that many', () => {
    const pins = ['mealplans', 'recipes', 'trains', 'books']
    expect(ids(defaultPins({ ...user, suggestedPins: pins }))).toEqual([
      'mealplans',
      'recipes',
      'trains'
    ])
  })

  it('skips suggestions the user can not pin or open, and duplicates', () => {
    const pins = ['settings', 'factory', 'movies', 'recipes', 'recipes']
    expect(ids(defaultPins({ ...user, suggestedPins: pins }))).toEqual([
      'recipes',
      'books',
      'feeds'
    ])
  })

  it('falls back to the fixed order without history', () => {
    expect(ids(defaultPins({ ...user, suggestedPins: [] }))).toEqual([
      'books',
      'feeds',
      'mealplans'
    ])
  })
})

describe('defaultPins', () => {
  it('prefers books, feeds and meal plans, in that order', () => {
    const user = { role: 'user', appAccess: ['mealplans', 'trains', 'feeds', 'books'] }
    expect(ids(defaultPins(user))).toEqual(['books', 'feeds', 'mealplans'])
  })

  it('fills the rest from other apps the user can open, never account or admin apps', () => {
    expect(
      ids(defaultPins({ role: 'user', appAccess: ['books', 'trains', 'movies', 'taste'] }))
    ).toEqual(['books', 'movies', 'taste'])
    expect(ids(defaultPins({ role: 'user', appAccess: [] }))).toEqual([])
  })

  it('caps at three for an admin', () => {
    expect(defaultPins({ role: 'admin' })).toHaveLength(3)
    expect(defaultPins({ role: 'admin' }).every((a) => !a.adminOnly)).toBe(true)
  })
})

describe('resolvePins', () => {
  const books = { role: 'user', appAccess: ['books', 'feeds', 'mealplans', 'trains'] }

  it('uses the default pins until the user chooses', () => {
    expect(ids(resolvePins({ ...books, tabPins: ['trains'] }))).toEqual([
      'books',
      'feeds',
      'mealplans'
    ])
  })

  it('keeps a saved choice in its order', () => {
    const user = { ...books, tabPins: ['trains', 'books'], tabPinsCustomised: true }
    expect(ids(resolvePins(user))).toEqual(['trains', 'books'])
  })

  it('treats a chosen but missing list as none', () => {
    expect(ids(resolvePins({ ...books, tabPinsCustomised: true }))).toEqual([])
  })

  it('honours a deliberate empty choice', () => {
    expect(ids(resolvePins({ ...books, tabPins: [], tabPinsCustomised: true }))).toEqual([])
  })

  it('replaces a pin the user lost access to with the next default', () => {
    const user = {
      role: 'user',
      appAccess: ['books', 'feeds'],
      tabPins: ['movies', 'books'],
      tabPinsCustomised: true
    }
    expect(ids(resolvePins(user))).toEqual(['books', 'feeds'])
  })

  it('ignores duplicates and never fills past the saved count', () => {
    const user = { ...books, tabPins: ['books', 'books', 'ghost'], tabPinsCustomised: true }
    expect(ids(resolvePins(user))).toEqual(['books', 'feeds'])
  })

  it('never pins the account entries', () => {
    const user = { ...books, tabPins: ['settings'], tabPinsCustomised: true }
    expect(ids(resolvePins(user))).toEqual(['books'])
  })

  it('lets an admin pin an admin app', () => {
    const user = { role: 'admin', tabPins: ['factory'], tabPinsCustomised: true }
    expect(ids(resolvePins(user))).toEqual(['factory'])
  })
})

describe('pinnableApps', () => {
  it('leaves out the account entries', () => {
    expect(ids(pinnableApps({ role: 'user', appAccess: ['books'] }))).toEqual(['books'])
  })
})
