import { Code, ConnectError } from '@connectrpc/connect'
import { isSignedOut, signInRedirect } from '@/lib/signIn'

const unauthenticated = () => new ConnectError('not signed in', Code.Unauthenticated)

describe('isSignedOut', () => {
  it('is true only for an Unauthenticated error from the api', () => {
    expect(isSignedOut(unauthenticated())).toBe(true)
    expect(isSignedOut(new ConnectError('down', Code.Unavailable))).toBe(false)
    expect(isSignedOut(new TypeError('Failed to fetch'))).toBe(false)
    expect(isSignedOut(undefined)).toBe(false)
  })
})

describe('signInRedirect', () => {
  it.each([
    ['/feeds', '', '/auth/sign-in?next=%2Ffeeds'],
    ['/books/42', 'tab=notes', '/auth/sign-in?next=%2Fbooks%2F42%3Ftab%3Dnotes'],
    ['/dashboard/games', '', '/auth/sign-in?next=%2Fdashboard%2Fgames'],
    ['/dashboard/reading', '', '/auth/sign-in?next=%2Fdashboard%2Freading']
  ])('sends a signed-out visitor on %s back via sign-in', (pathname, search, href) => {
    expect(signInRedirect(unauthenticated(), pathname, search)).toBe(href)
  })

  it.each([
    '/',
    '/auth/sign-in',
    '/auth/forgot-password',
    '/auth/reset-password',
    '/oauth/consent',
    '/dashboard/games/tok',
    '/dashboard/games/tok/library',
    '/dashboard/games/tok/42',
    '/dashboard/reading/tok',
    '/dashboard/reading/tok/library'
  ])('lets a signed-out visitor stay on the public page %s', (pathname) => {
    expect(signInRedirect(unauthenticated(), pathname, '')).toBeNull()
  })

  it.each([
    ['no error', undefined],
    ['a network error', new TypeError('Failed to fetch')],
    ['an unreachable api', new ConnectError('down', Code.Unavailable)],
    ['a server error', new ConnectError('boom', Code.Internal)]
  ])('stays put on %s', (_, error) => {
    expect(signInRedirect(error, '/feeds', '')).toBeNull()
  })
})
