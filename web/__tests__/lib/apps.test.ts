import { APPS, artworkApp, visibleApps, visibleSections } from '@/lib/apps'

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
