import { hidesTabBar } from '@/lib/shell'

describe('hidesTabBar', () => {
  it.each(['/books/42/read', '/dashboard/reading/tok', '/books/42/read/'])(
    'hides on %s',
    (path) => {
      expect(hidesTabBar(path)).toBe(true)
    }
  )

  it.each([
    '/',
    '/apps',
    '/books/42',
    '/books/library',
    '/dashboard/reading',
    '/books/42/reader',
    '/x/books/42/read',
    '/books//read'
  ])('shows on %s', (path) => {
    expect(hidesTabBar(path)).toBe(false)
  })
})
