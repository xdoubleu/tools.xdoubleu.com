import { isManagedEndpoint, KOBO_DEFAULT_ENDPOINT } from '@/lib/books/koboConf'

describe('KOBO_DEFAULT_ENDPOINT', () => {
  it('is the stock Kobo store URL', () => {
    expect(KOBO_DEFAULT_ENDPOINT).toBe('https://storeapi.kobo.com')
  })
})

describe('isManagedEndpoint', () => {
  const apiUrl = 'https://myserver'

  it('returns true when the endpoint is under this server’s Kobo sync path', () => {
    expect(isManagedEndpoint('https://myserver/books/kobo/TOKEN', apiUrl)).toBe(true)
  })

  it('returns false for the stock Kobo store endpoint', () => {
    expect(isManagedEndpoint(KOBO_DEFAULT_ENDPOINT, apiUrl)).toBe(false)
  })

  it('returns false for a different server’s sync path', () => {
    expect(isManagedEndpoint('https://otherserver/books/kobo/TOKEN', apiUrl)).toBe(false)
  })

  it('returns false when the endpoint is undefined', () => {
    expect(isManagedEndpoint(undefined, apiUrl)).toBe(false)
  })
})
