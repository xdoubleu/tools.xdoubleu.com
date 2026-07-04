import { serverFetch, createServerClient } from '@/lib/server/client'
import { AuthService } from '@/lib/gen/auth/v1/auth_pb'

jest.mock('next/headers', () => ({
  cookies: jest.fn(async () => ({
    toString: () => 'accessToken=abc; refreshToken=def'
  }))
}))

// eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
const fakeResponse = { ok: true } as Response

function mockGlobalFetch() {
  const mockFetch = jest.fn(async () => fakeResponse)
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
  global.fetch = mockFetch as unknown as typeof fetch
  return mockFetch
}

function callArgs(mockFetch: jest.Mock): [string, RequestInit] {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
  return mockFetch.mock.calls[0] as unknown as [string, RequestInit]
}

describe('serverFetch', () => {
  const realFetch = global.fetch

  afterEach(() => {
    global.fetch = realFetch
  })

  it('forwards the cookie header and disables caching', async () => {
    const mockFetch = mockGlobalFetch()

    await serverFetch('accessToken=abc')('http://api.test/x', {
      method: 'POST',
      headers: { 'content-type': 'application/proto' }
    })

    expect(mockFetch).toHaveBeenCalledTimes(1)
    const [url, init] = callArgs(mockFetch)
    expect(url).toBe('http://api.test/x')
    const headers = new Headers(init.headers)
    expect(headers.get('cookie')).toBe('accessToken=abc')
    expect(headers.get('content-type')).toBe('application/proto')
    expect(init.cache).toBe('no-store')
  })

  it('omits the cookie header when there are no cookies', async () => {
    const mockFetch = mockGlobalFetch()

    await serverFetch('')('http://api.test/x')

    const [, init] = callArgs(mockFetch)
    expect(new Headers(init.headers).get('cookie')).toBeNull()
  })
})

describe('createServerClient', () => {
  it('builds a client for the given service', async () => {
    const client = await createServerClient(AuthService)
    expect(typeof client.getCurrentUser).toBe('function')
  })
})
