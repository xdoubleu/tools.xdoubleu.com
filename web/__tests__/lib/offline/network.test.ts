import { Code, ConnectError } from '@connectrpc/connect'
import { isNetworkError } from '@/lib/offline/network'

describe('isNetworkError', () => {
  afterEach(() => {
    Object.defineProperty(navigator, 'onLine', { value: true, configurable: true })
  })

  it('classifies errors', () => {
    const fetchFailure = new ConnectError(
      'Failed to fetch',
      Code.Unknown,
      undefined,
      undefined,
      new TypeError('Failed to fetch')
    )
    expect(isNetworkError(new TypeError('Failed to fetch'))).toBe(true)
    expect(isNetworkError(fetchFailure)).toBe(true)
    expect(isNetworkError(new ConnectError('down', Code.Unavailable))).toBe(true)
    expect(isNetworkError(new ConnectError('nope', Code.NotFound))).toBe(false)
    expect(isNetworkError(new Error('boom'))).toBe(false)
  })

  it('treats any error as a network error while the browser is offline', () => {
    Object.defineProperty(navigator, 'onLine', { value: false, configurable: true })
    expect(isNetworkError(new Error('boom'))).toBe(true)
  })
})
