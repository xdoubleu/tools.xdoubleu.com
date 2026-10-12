import { serverFetch, createServerClient } from '@/lib/server/client'
import { AuthService } from '@/lib/gen/auth/v1/auth_pb'

jest.mock('next/headers', () => ({
  cookies: jest.fn(async () => ({
    getAll: () => [
      { name: 'accessToken', value: 'abc' },
      { name: 'refreshToken', value: 'def' }
    ]
  }))
}))

// eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
const fakeResponse = { ok: true } as Response

// eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
const fakeResponse502 = { ok: false, status: 502 } as Response

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

  it('retries on a 502 (api mid-restart) and returns the success response', async () => {
    const mockFetch = jest
      .fn()
      .mockResolvedValueOnce(fakeResponse502)
      .mockResolvedValueOnce(fakeResponse)
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
    global.fetch = mockFetch as unknown as typeof fetch

    const res = await serverFetch('')('http://api.test/x')

    expect(mockFetch).toHaveBeenCalledTimes(2)
    expect(res.ok).toBe(true)
  })

  it('gives up after the bounded retries and returns the last 502', async () => {
    const mockFetch = jest.fn(async () => fakeResponse502)
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
    global.fetch = mockFetch as unknown as typeof fetch

    const res = await serverFetch('')('http://api.test/x')

    expect(mockFetch).toHaveBeenCalledTimes(2 + 1)
    expect(res.status).toBe(502)
  })

  it('does not retry non-502 failures', async () => {
    const mockFetch = jest.fn(
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
      async () => ({ ok: false, status: 404 }) as Response
    )
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
    global.fetch = mockFetch as unknown as typeof fetch

    await serverFetch('')('http://api.test/x')

    expect(mockFetch).toHaveBeenCalledTimes(1)
  })
})

describe('createServerClient', () => {
  it('builds a client for the given service', async () => {
    const client = await createServerClient(AuthService)
    expect(typeof client.getCurrentUser).toBe('function')
  })
})
