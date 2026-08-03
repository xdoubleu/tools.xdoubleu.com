import {
  probeGateway,
  configureGateway,
  revertGateway,
  updateGateway,
  gatewayNeedsUpdate,
  REQUIRED_GATEWAY_VERSION,
  type GatewayStatus
} from '@/lib/books/gatewayClient'

// jsdom may not ship AbortSignal.timeout; the real targets (modern browsers)
// all do.
if (typeof AbortSignal.timeout !== 'function') {
  Object.defineProperty(AbortSignal, 'timeout', {
    value: () => new AbortController().signal,
    configurable: true
  })
}

const mockFetch = jest.fn()

beforeEach(() => {
  mockFetch.mockReset()
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
  global.fetch = mockFetch as unknown as typeof fetch
})

function jsonResponse(body: unknown, ok = true, status = 200) {
  return {
    ok,
    status,
    json: () => Promise.resolve(body)
  }
}

const SAMPLE_STATUS = {
  version: 1,
  release: 'abc1234',
  kobos: [
    {
      volumePath: '/Volumes/KOBOeReader',
      serial: 'N418ABCD1234',
      currentEndpoint: 'https://storeapi.kobo.com'
    }
  ]
}

describe('probeGateway', () => {
  it('returns the status when the gateway responds', async () => {
    mockFetch.mockResolvedValue(jsonResponse(SAMPLE_STATUS))

    const status = await probeGateway()

    expect(status).toEqual(SAMPLE_STATUS)
    expect(mockFetch).toHaveBeenCalledWith(
      'https://127.0.0.1:41132/status',
      expect.objectContaining({ signal: expect.anything() })
    )
  })

  it('returns null when the fetch fails (gateway not running)', async () => {
    mockFetch.mockRejectedValue(new TypeError('fetch failed'))

    expect(await probeGateway()).toBeNull()
  })

  it('returns null on a non-OK response', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: 'nope' }, false, 403))

    expect(await probeGateway()).toBeNull()
  })
})

describe('configureGateway', () => {
  it('POSTs the sync URL and returns serial + original endpoint', async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ serial: 'N418ABCD1234', originalEndpoint: 'https://storeapi.kobo.com' })
    )

    const res = await configureGateway('https://api.example.com/books/kobo/TOKEN')

    expect(res).toEqual({
      serial: 'N418ABCD1234',
      originalEndpoint: 'https://storeapi.kobo.com'
    })
    expect(mockFetch).toHaveBeenCalledWith('https://127.0.0.1:41132/configure', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ syncUrl: 'https://api.example.com/books/kobo/TOKEN' })
    })
  })

  it('includes volumePath when given', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ serial: 'S', originalEndpoint: '' }))

    await configureGateway('https://api.example.com/books/kobo/TOKEN', '/Volumes/KOBO2')

    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
    const body = JSON.parse((mockFetch.mock.calls[0][1] as RequestInit).body as string)
    expect(body.volumePath).toBe('/Volumes/KOBO2')
  })

  it('surfaces the gateway error message on failure', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: 'no Kobo volume found' }, false, 404))

    await expect(configureGateway('https://api.example.com/books/kobo/TOKEN')).rejects.toThrow(
      'no Kobo volume found'
    )
  })

  it('falls back to a status message when the error body is not JSON', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.reject(new Error('not json'))
    })

    await expect(configureGateway('https://api.example.com/books/kobo/TOKEN')).rejects.toThrow(
      'Gateway request failed (500)'
    )
  })
})

describe('revertGateway', () => {
  it('POSTs the target endpoint', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ serial: 'N418ABCD1234' }))

    const res = await revertGateway('https://storeapi.kobo.com')

    expect(res).toEqual({ serial: 'N418ABCD1234' })
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
    const body = JSON.parse((mockFetch.mock.calls[0][1] as RequestInit).body as string)
    expect(body).toEqual({ targetEndpoint: 'https://storeapi.kobo.com' })
  })
})

describe('updateGateway', () => {
  it('POSTs to /update', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ updating: true }))

    const res = await updateGateway()

    expect(res).toEqual({ updating: true })
    expect(mockFetch).toHaveBeenCalledWith(
      'https://127.0.0.1:41132/update',
      expect.objectContaining({ method: 'POST' })
    )
  })
})

describe('REQUIRED_GATEWAY_VERSION', () => {
  it('is a positive integer', () => {
    expect(REQUIRED_GATEWAY_VERSION).toBeGreaterThanOrEqual(1)
  })
})

describe('gatewayNeedsUpdate', () => {
  function status(overrides: Partial<GatewayStatus> = {}): GatewayStatus {
    return { version: REQUIRED_GATEWAY_VERSION, release: 'abc1234', kobos: [], ...overrides }
  }

  it('is true when the protocol version is below the required minimum', () => {
    window.__ENV__ = { API_URL: '', SENTRY_DSN: '', RELEASE: 'abc1234' }

    expect(gatewayNeedsUpdate(status({ version: REQUIRED_GATEWAY_VERSION - 1 }))).toBe(true)
  })

  it('is true when the release does not match this web build', () => {
    window.__ENV__ = { API_URL: '', SENTRY_DSN: '', RELEASE: 'current-sha' }

    expect(gatewayNeedsUpdate(status({ release: 'stale-sha' }))).toBe(true)
  })

  it('is false when version and release both match', () => {
    window.__ENV__ = { API_URL: '', SENTRY_DSN: '', RELEASE: 'abc1234' }

    expect(gatewayNeedsUpdate(status({ release: 'abc1234' }))).toBe(false)
  })

  it('is false when this web build is a local dev build', () => {
    window.__ENV__ = { API_URL: '', SENTRY_DSN: '', RELEASE: 'dev' }

    expect(gatewayNeedsUpdate(status({ release: 'stale-sha' }))).toBe(false)
  })

  it('is false when the gateway itself is a local dev build', () => {
    window.__ENV__ = { API_URL: '', SENTRY_DSN: '', RELEASE: 'current-sha' }

    expect(gatewayNeedsUpdate(status({ release: 'dev' }))).toBe(false)
  })
})
