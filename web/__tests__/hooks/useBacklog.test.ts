import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
// Return a stable fake checksum so tests don't depend on real crypto.subtle.
jest.mock('@/lib/backlog/checksum', () => ({
  sha256Hex: jest.fn().mockResolvedValue('aabbccdd')
}))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    getSteamGame: jest.fn().mockResolvedValue({}),
    getSteamDistribution: jest.fn().mockResolvedValue({}),
    getSteam: jest.fn().mockResolvedValue({}),
    getRecentlyActiveGames: jest.fn().mockResolvedValue({}),
    getBooksProgress: jest.fn().mockResolvedValue({}),
    importBooks: jest.fn().mockResolvedValue({}),
    createBookUpload: jest.fn().mockResolvedValue({
      uploadId: 'users/u1/uploads/uuid.epub',
      url: 'https://r2.example.com/put',
      alreadyExists: false
    }),
    finalizeBookUpload: jest.fn().mockResolvedValue({}),
    enableKoboSync: jest.fn().mockResolvedValue({ kepubStatus: 'converting' }),
    requestKEPUBConversion: jest.fn().mockResolvedValue({ kepubStatus: 'converting' }),
    getKEPUBStatus: jest.fn().mockResolvedValue({ hasEpub: true, kepubStatus: 'ready' }),
    registerKoboDevice: jest.fn().mockResolvedValue({ device: { id: 'd1' }, rawToken: 'abc123' }),
    listKoboDevices: jest.fn().mockResolvedValue({ devices: [] }),
    disconnectKoboDevice: jest.fn().mockResolvedValue({}),
    getBookFile: jest.fn().mockResolvedValue({ url: 'https://r2.example.com/file.pdf' })
  }))
}))
jest.mock('@/lib/gen/backlog/v1/books_pb', () => ({ BooksService: {} }))
jest.mock('@/lib/gen/backlog/v1/games_pb', () => ({ GamesService: {} }))
jest.mock('@/lib/env', () => ({ getApiUrl: () => 'https://api.test' }))

import useSWR from 'swr'
import {
  useBacklogLibrary,
  useBacklogSteam,
  useBacklogSteamGame,
  useBacklogDistribution,
  useBooksProgress,
  useSteamProgress,
  useRecentlyActiveGames,
  useRefreshSteam,
  useSearchExternal,
  useAddBook,
  useImportBooks,
  useUpdateBookStatus,
  useToggleTag,
  useUploadBookFile,
  useEnableKoboSync,
  useRequestKEPUBConversion,
  useKEPUBStatus,
  useGetBookFile,
  useRegisterKoboDevice,
  useListKoboDevices,
  useDisconnectKoboDevice
} from '@/hooks/useBacklog'
import { createServiceClient } from '@/lib/client'

const mockUseSWR = jest.mocked(useSWR)
const mockCreateServiceClient = jest.mocked(createServiceClient)

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
  mockCreateServiceClient.mockClear()
})

describe('useBacklogLibrary', () => {
  it('uses /backlog/books as key', () => {
    renderHook(() => useBacklogLibrary())
    expect(mockUseSWR).toHaveBeenCalledWith('/backlog/books', expect.any(Function))
  })
})

describe('useBacklogSteam', () => {
  it('uses /backlog/games as key', () => {
    renderHook(() => useBacklogSteam())
    expect(mockUseSWR).toHaveBeenCalledWith('/backlog/games', expect.any(Function))
  })
})

describe('useRecentlyActiveGames', () => {
  it('uses /backlog/games/recent as key', () => {
    renderHook(() => useRecentlyActiveGames())
    expect(mockUseSWR).toHaveBeenCalledWith('/backlog/games/recent', expect.any(Function))
  })

  it('fetcher calls client.getRecentlyActiveGames', async () => {
    const mockClient = { getRecentlyActiveGames: jest.fn().mockResolvedValue({}) }
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce(mockClient)
    renderHook(() => useRecentlyActiveGames())
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockClient.getRecentlyActiveGames).toHaveBeenCalledWith({})
  })
})

describe('useBacklogSteamGame', () => {
  it('uses correct key when gameId is provided', () => {
    renderHook(() => useBacklogSteamGame(12345))
    expect(mockUseSWR).toHaveBeenCalledWith('/backlog/games/12345', expect.any(Function))
  })

  it('passes null as key when gameId is 0', () => {
    renderHook(() => useBacklogSteamGame(0))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })

  it('fetcher calls client.getSteamGame', async () => {
    const mockClient = { getSteamGame: jest.fn().mockResolvedValue({}) }
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce(mockClient)
    renderHook(() => useBacklogSteamGame(42))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockClient.getSteamGame).toHaveBeenCalledWith({ gameId: 42 })
  })
})

describe('useBacklogDistribution', () => {
  it('uses /backlog/games/distribution/${bucket} as key', () => {
    renderHook(() => useBacklogDistribution(10))
    expect(mockUseSWR).toHaveBeenCalledWith('/backlog/games/distribution/10', expect.any(Function))
  })

  it('fetcher calls client.getSteamDistribution', async () => {
    const mockClient = { getSteamDistribution: jest.fn().mockResolvedValue({}) }
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce(mockClient)
    renderHook(() => useBacklogDistribution(10))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockClient.getSteamDistribution).toHaveBeenCalledWith({ bucket: 10 })
  })
})

describe('useSteamProgress', () => {
  it('uses correct key with dates', () => {
    renderHook(() => useSteamProgress('2024-01-01', '2024-12-31'))
    const [key] = mockUseSWR.mock.calls[0]
    expect(key).toEqual(['/backlog/games/progress', '2024-01-01', '2024-12-31'])
  })

  it('uses key with undefined dates', () => {
    renderHook(() => useSteamProgress())
    const [key] = mockUseSWR.mock.calls[0]
    expect(key).toEqual(['/backlog/games/progress', undefined, undefined])
  })

  it('fetcher calls client.getSteam with dates', async () => {
    const mockClient = { getSteam: jest.fn().mockResolvedValue({}) }
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce(mockClient)
    renderHook(() => useSteamProgress('2024-01-01', '2024-12-31'))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockClient.getSteam).toHaveBeenCalledWith({
      dateStart: '2024-01-01',
      dateEnd: '2024-12-31'
    })
  })
})

describe('useBooksProgress', () => {
  it('uses correct key with date range', () => {
    renderHook(() => useBooksProgress('2024-01-01', '2024-12-31'))
    const [key] = mockUseSWR.mock.calls[0]
    expect(key).toEqual(['/backlog/books/progress', '2024-01-01', '2024-12-31'])
  })

  it('passes null as key when no dates provided', () => {
    renderHook(() => useBooksProgress())
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })

  it('fetcher calls client.getBooksProgress', async () => {
    const mockClient = { getBooksProgress: jest.fn().mockResolvedValue({}) }
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce(mockClient)
    renderHook(() => useBooksProgress('2024-01-01', '2024-12-31'))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockClient.getBooksProgress).toHaveBeenCalledWith({
      dateStart: '2024-01-01',
      dateEnd: '2024-12-31'
    })
  })
})

describe('useRefreshSteam', () => {
  it('fetches the steam refresh endpoint with credentials', async () => {
    global.fetch = jest.fn().mockResolvedValue({})
    const { result } = renderHook(() => useRefreshSteam())
    await result.current()
    expect(global.fetch).toHaveBeenCalledWith(
      'https://api.test/backlog/api/progress/steam/refresh',
      { credentials: 'include' }
    )
  })
})

describe('useSearchExternal', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useSearchExternal())
    expect(typeof result.current).toBe('function')
  })
})

describe('useAddBook', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useAddBook())
    expect(typeof result.current).toBe('function')
  })
})

describe('useImportBooks', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useImportBooks())
    expect(typeof result.current).toBe('function')
  })

  it('encodes csv and calls client.importBooks', () => {
    const mockImportBooks = jest.fn().mockResolvedValue({})
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce({ importBooks: mockImportBooks })
    const { result } = renderHook(() => useImportBooks())
    result.current('a,b\n1,2')
    const call = mockImportBooks.mock.calls[0][0]
    expect(Object.prototype.toString.call(call.csvData)).toBe('[object Uint8Array]')
  })
})

describe('useUpdateBookStatus', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useUpdateBookStatus())
    expect(typeof result.current).toBe('function')
  })
})

describe('useToggleTag', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useToggleTag())
    expect(typeof result.current).toBe('function')
  })
})

describe('useUploadBookFile', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useUploadBookFile())
    expect(typeof result.current).toBe('function')
  })

  it('sends checksum in createBookUpload and performs PUT then finalizeBookUpload', async () => {
    const mockCreate = jest.fn().mockResolvedValue({
      uploadId: 'users/u1/uploads/uuid.epub',
      url: 'https://r2.example.com/put',
      alreadyExists: false
    })
    const mockFinalize = jest.fn().mockResolvedValue({})
    const partialClient = { createBookUpload: mockCreate, finalizeBookUpload: mockFinalize }
    // @ts-expect-error -- partial mock client; only upload methods needed for this test
    mockCreateServiceClient.mockReturnValueOnce(partialClient)
    global.fetch = jest.fn().mockResolvedValue({ ok: true })

    const { result } = renderHook(() => useUploadBookFile())
    const file = new File(['data'], 'book.epub', { type: 'application/epub+zip' })
    await result.current(file)

    // The client-computed checksum must be forwarded to both RPCs.
    expect(mockCreate).toHaveBeenCalledWith({
      filename: 'book.epub',
      contentType: 'application/epub+zip',
      size: BigInt(file.size),
      checksum: 'aabbccdd'
    })
    expect(global.fetch).toHaveBeenCalledWith('https://r2.example.com/put', {
      method: 'PUT',
      body: file,
      headers: { 'Content-Type': 'application/epub+zip' }
    })
    expect(mockFinalize).toHaveBeenCalledWith({
      uploadId: 'users/u1/uploads/uuid.epub',
      filename: 'book.epub',
      contentType: 'application/epub+zip',
      checksum: 'aabbccdd'
    })
  })

  it('skips the PUT when server reports alreadyExists', async () => {
    const mockCreate = jest.fn().mockResolvedValue({
      uploadId: '',
      url: '',
      alreadyExists: true
    })
    const mockFinalize = jest.fn().mockResolvedValue({})
    const partialClient = { createBookUpload: mockCreate, finalizeBookUpload: mockFinalize }
    // @ts-expect-error -- partial mock client; only upload methods needed for this test
    mockCreateServiceClient.mockReturnValueOnce(partialClient)
    global.fetch = jest.fn()

    const { result } = renderHook(() => useUploadBookFile())
    const file = new File(['data'], 'book.epub', { type: 'application/epub+zip' })
    await result.current(file)

    // No PUT to R2 when alreadyExists.
    expect(global.fetch).not.toHaveBeenCalled()
    // Finalize is still called so the server can wire up the user's library entry.
    expect(mockFinalize).toHaveBeenCalledWith(expect.objectContaining({ checksum: 'aabbccdd' }))
  })

  it('throws when the R2 PUT fails', async () => {
    const mockCreate = jest.fn().mockResolvedValue({
      uploadId: 'users/u1/uploads/uuid.epub',
      url: 'https://r2.example.com/put',
      alreadyExists: false
    })
    const partialClient = { createBookUpload: mockCreate, finalizeBookUpload: jest.fn() }
    // @ts-expect-error -- partial mock client; only upload methods needed for this test
    mockCreateServiceClient.mockReturnValueOnce(partialClient)
    global.fetch = jest.fn().mockResolvedValue({ ok: false, status: 403 })

    const { result } = renderHook(() => useUploadBookFile())
    const file = new File(['data'], 'book.epub', { type: 'application/epub+zip' })
    await expect(result.current(file)).rejects.toThrow('Upload to storage failed (403)')
  })
})

describe('useEnableKoboSync', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useEnableKoboSync())
    expect(typeof result.current).toBe('function')
  })

  it('calls client.enableKoboSync with bookId', () => {
    const mockEnable = jest.fn().mockResolvedValue({ kepubStatus: 'converting' })
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce({ enableKoboSync: mockEnable })
    const { result } = renderHook(() => useEnableKoboSync())
    result.current('book-123')
    expect(mockEnable).toHaveBeenCalledWith({ bookId: 'book-123' })
  })
})

describe('useRegisterKoboDevice', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useRegisterKoboDevice())
    expect(typeof result.current).toBe('function')
  })

  it('calls client.registerKoboDevice with name and serial', async () => {
    const mockRegister = jest
      .fn()
      .mockResolvedValue({ device: { id: 'dev-1' }, rawToken: 'tok123' })
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce({ registerKoboDevice: mockRegister })
    const { result } = renderHook(() => useRegisterKoboDevice())
    const res = await result.current('My Kobo', 'SN1234')
    expect(mockRegister).toHaveBeenCalledWith({ name: 'My Kobo', serial: 'SN1234' })
    expect(res.rawToken).toBe('tok123')
  })
})

describe('useListKoboDevices', () => {
  it('uses /backlog/kobo/devices as SWR key', () => {
    renderHook(() => useListKoboDevices())
    expect(mockUseSWR).toHaveBeenCalledWith('/backlog/kobo/devices', expect.any(Function))
  })

  it('fetcher calls client.listKoboDevices', async () => {
    const mockList = jest.fn().mockResolvedValue({ devices: [] })
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce({ listKoboDevices: mockList })
    renderHook(() => useListKoboDevices())
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockList).toHaveBeenCalledWith({})
  })
})

describe('useDisconnectKoboDevice', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useDisconnectKoboDevice())
    expect(typeof result.current).toBe('function')
  })

  it('calls client.disconnectKoboDevice with id', () => {
    const mockDisconnect = jest.fn().mockResolvedValue({})
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce({ disconnectKoboDevice: mockDisconnect })
    const { result } = renderHook(() => useDisconnectKoboDevice())
    result.current('dev-id-123')
    expect(mockDisconnect).toHaveBeenCalledWith({ id: 'dev-id-123' })
  })
})

describe('useKEPUBStatus', () => {
  it('uses null key when bookId is null', () => {
    renderHook(() => useKEPUBStatus(null))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function), expect.any(Object))
  })

  it('uses composite key when bookId is provided', () => {
    renderHook(() => useKEPUBStatus('book-abc'))
    expect(mockUseSWR).toHaveBeenCalledWith(
      ['/backlog/books/kepub-status', 'book-abc'],
      expect.any(Function),
      expect.any(Object)
    )
  })

  it('fetcher calls client.getKEPUBStatus', async () => {
    const mockGetStatus = jest.fn().mockResolvedValue({ hasEpub: true, kepubStatus: 'ready' })
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce({ getKEPUBStatus: mockGetStatus })
    renderHook(() => useKEPUBStatus('book-abc'))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockGetStatus).toHaveBeenCalledWith({ bookId: 'book-abc' })
  })
})

describe('useGetBookFile', () => {
  it('uses null key when bookId is null', () => {
    renderHook(() => useGetBookFile(null, 'pdf'))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })

  it('uses null key when format is null', () => {
    renderHook(() => useGetBookFile('book-abc', null))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })

  it('uses composite key when both bookId and format are provided', () => {
    renderHook(() => useGetBookFile('book-abc', 'pdf'))
    expect(mockUseSWR).toHaveBeenCalledWith(
      ['/backlog/books/file', 'book-abc', 'pdf'],
      expect.any(Function)
    )
  })

  it('fetcher calls client.getBookFile with bookId and format', async () => {
    const mockGetFile = jest.fn().mockResolvedValue({ url: 'https://r2.example.com/file.pdf' })
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce({ getBookFile: mockGetFile })
    renderHook(() => useGetBookFile('book-abc', 'pdf'))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockGetFile).toHaveBeenCalledWith({ bookId: 'book-abc', format: 'pdf' })
  })

  it('fetcher calls client.getBookFile with epub format', async () => {
    const mockGetFile = jest.fn().mockResolvedValue({ url: 'https://r2.example.com/file.epub' })
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce({ getBookFile: mockGetFile })
    renderHook(() => useGetBookFile('book-xyz', 'epub'))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockGetFile).toHaveBeenCalledWith({ bookId: 'book-xyz', format: 'epub' })
  })
})

describe('useRequestKEPUBConversion', () => {
  it('returns a function that calls client.requestKEPUBConversion', async () => {
    const mockConvert = jest.fn().mockResolvedValue({ kepubStatus: 'converting' })
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce({ requestKEPUBConversion: mockConvert })
    const { result } = renderHook(() => useRequestKEPUBConversion())
    await result.current('book-123')
    expect(mockConvert).toHaveBeenCalledWith({ bookId: 'book-123' })
  })
})
