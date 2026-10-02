import { render, screen, waitFor } from '@testing-library/react'
import useSWR, { SWRConfig, unstable_serialize } from 'swr'
import { Code, ConnectError } from '@connectrpc/connect'
import { isNetworkError, persistMiddleware } from '@/lib/offline/persist'
import { loadEntry, saveEntry } from '@/lib/offline/store'
import { getSnapshot, markLive } from '@/lib/offline/status'

jest.mock('@/lib/offline/store', () => ({
  loadEntry: jest.fn(),
  saveEntry: jest.fn()
}))

const mockLoad = jest.mocked(loadEntry)
const mockSave = jest.mocked(saveEntry)

function Probe({
  swrKey,
  fetcher
}: {
  swrKey: string | [string, string]
  fetcher: () => Promise<string>
}) {
  const { data, error } = useSWR<string, Error>(swrKey, fetcher)
  if (error) return <span>error:{error.message}</span>
  return <span>{data ?? 'loading'}</span>
}

function renderProbe(swrKey: string | [string, string], fetcher: () => Promise<string>) {
  return render(
    <SWRConfig
      value={{ provider: () => new Map(), use: [persistMiddleware], shouldRetryOnError: false }}
    >
      <Probe swrKey={swrKey} fetcher={fetcher} />
    </SWRConfig>
  )
}

const networkError = () =>
  new ConnectError(
    'Failed to fetch',
    Code.Unknown,
    undefined,
    undefined,
    new TypeError('Failed to fetch')
  )

describe('persistMiddleware', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    markLive()
  })

  it('saves successful responses under the serialized key', async () => {
    renderProbe(['/feeds', 'x'], () => Promise.resolve('live'))

    expect(await screen.findByText('live')).toBeInTheDocument()
    expect(mockSave).toHaveBeenCalledWith(unstable_serialize(['/feeds', 'x']), 'live')
  })

  it('serves the saved copy on a network error', async () => {
    mockLoad.mockResolvedValue({ data: 'saved', savedAt: 42 })
    renderProbe('/recipes', () => Promise.reject(networkError()))

    expect(await screen.findByText('saved')).toBeInTheDocument()
    expect(mockLoad).toHaveBeenCalledWith(unstable_serialize('/recipes'))
    expect(getSnapshot().servedFromCacheAt).toBe(42)
  })

  it('surfaces the network error when nothing was saved', async () => {
    mockLoad.mockResolvedValue(undefined)
    renderProbe('/recipes', () => Promise.reject(networkError()))

    expect(await screen.findByText('error:[unknown] Failed to fetch')).toBeInTheDocument()
  })

  it('surfaces server errors without consulting the store', async () => {
    renderProbe('/recipes', () => Promise.reject(new ConnectError('nope', Code.PermissionDenied)))

    expect(await screen.findByText('error:[permission_denied] nope')).toBeInTheDocument()
    expect(mockLoad).not.toHaveBeenCalled()
  })

  it('neither saves nor falls back for excluded keys', async () => {
    renderProbe('/trains/journey', () => Promise.resolve('live'))
    expect(await screen.findByText('live')).toBeInTheDocument()

    renderProbe('/release', () => Promise.reject(networkError()))
    await waitFor(() => expect(screen.getByText(/error:/)).toBeInTheDocument())

    expect(mockSave).not.toHaveBeenCalled()
    expect(mockLoad).not.toHaveBeenCalled()
  })
})

describe('isNetworkError', () => {
  afterEach(() => {
    Object.defineProperty(navigator, 'onLine', { value: true, configurable: true })
  })

  it('classifies errors', () => {
    expect(isNetworkError(new TypeError('Failed to fetch'))).toBe(true)
    expect(isNetworkError(networkError())).toBe(true)
    expect(isNetworkError(new ConnectError('down', Code.Unavailable))).toBe(true)
    expect(isNetworkError(new ConnectError('nope', Code.NotFound))).toBe(false)
    expect(isNetworkError(new Error('boom'))).toBe(false)
  })

  it('treats any error as a network error while the browser is offline', () => {
    Object.defineProperty(navigator, 'onLine', { value: false, configurable: true })
    expect(isNetworkError(new Error('boom'))).toBe(true)
  })
})
