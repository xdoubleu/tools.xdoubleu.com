import { hidesTabBar } from '@/lib/shell'

describe('hidesTabBar', () => {
  it.each([
    '/books/42/read',
    '/watchparty/abc/presenter',
    '/dashboard/reading/tok',
    '/books/42/read/'
  ])('hides on %s', (path) => {
    expect(hidesTabBar(path)).toBe(true)
  })

  it.each([
    '/',
    '/apps',
    '/books/42',
    '/books/library',
    '/watchparty/abc',
    '/dashboard/reading',
    '/books/42/reader',
    '/x/books/42/read',
    '/x/watchparty/abc/presenter',
    '/books//read',
    '/watchparty//presenter',
    '/watchparty/abc/presenters'
  ])('shows on %s', (path) => {
    expect(hidesTabBar(path)).toBe(false)
  })
})
