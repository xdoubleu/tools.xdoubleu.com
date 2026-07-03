import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
// Return a stable fake checksum so tests don't depend on real crypto.subtle.
jest.mock('@/lib/books/checksum', () => ({
  sha256Hex: jest.fn().mockResolvedValue('aabbccdd')
}))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
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
    getBookFile: jest.fn().mockResolvedValue({ url: 'https://r2.example.com/file.pdf' }),
    searchLibrary: jest.fn().mockResolvedValue({ books: [] }),
    searchExternal: jest.fn().mockResolvedValue({ results: [] }),
    setBookISBN: jest.fn().mockResolvedValue({})
  }))
}))
jest.mock('@/lib/gen/books/v1/library_pb', () => ({
  LibraryService: {},
  CreateBookRequestSchema: {},
  UpdateBookStatusRequestSchema: {},
  UpdateProgressRequestSchema: {}
}))
jest.mock('@/lib/gen/books/v1/files_pb', () => ({ BookFilesService: {} }))
jest.mock('@/lib/gen/books/v1/kobo_pb', () => ({ KoboService: {} }))
jest.mock('@/lib/gen/books/v1/catalog_pb', () => ({ CatalogService: {} }))
jest.mock('@/lib/env', () => ({ getApiUrl: () => 'https://api.test' }))

import useSWR from 'swr'
import {
  useLibrary,
  useBooksProgress,
  useSearchLibrary,
  useSearchExternal,
  useCreateBook,
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
  useDisconnectKoboDevice,
  useSetBookISBN
} from '@/hooks/useBooks'
import { createServiceClient } from '@/lib/client'

const mockUseSWR = jest.mocked(useSWR)
const mockCreateServiceClient = jest.mocked(createServiceClient)

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
  mockCreateServiceClient.mockClear()
})

describe('useLibrary', () => {
  it('uses /books as key', () => {
    renderHook(() => useLibrary())
    expect(mockUseSWR).toHaveBeenCalledWith('/books', expect.any(Function))
  })
})

describe('useBooksProgress', () => {
  it('uses correct key with date range', () => {
    renderHook(() => useBooksProgress('2024-01-01', '2024-12-31'))
    const [key] = mockUseSWR.mock.calls[0]
    expect(key).toEqual(['/books/progress', '2024-01-01', '2024-12-31'])
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

describe('useSearchLibrary', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useSearchLibrary())
    expect(typeof result.current).toBe('function')
  })

  it('returns a stable function reference across re-renders', () => {
    // Regression test: before the fix, useSearchLibrary returned a new function
    // every render, causing an infinite effect loop in BookSearchBar that
    // swallowed Next.js <Link> navigation until the user typed something.
    const { result, rerender } = renderHook(() => useSearchLibrary())
    const first = result.current
    rerender()
    const second = result.current
    expect(Object.is(first, second)).toBe(true)
  })
})

describe('useSearchExternal', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useSearchExternal())
    expect(typeof result.current).toBe('function')
  })

  it('returns a stable function reference across re-renders', () => {
    // Same regression as useSearchLibrary — both hooks were unstable before the fix.
    const { result, rerender } = renderHook(() => useSearchExternal())
    const first = result.current
    rerender()
    const second = result.current
    expect(Object.is(first, second)).toBe(true)
  })
})

describe('useSetBookISBN', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useSetBookISBN())
    expect(typeof result.current).toBe('function')
  })

  it('returns a stable function reference across re-renders', () => {
    const { result, rerender } = renderHook(() => useSetBookISBN())
    const first = result.current
    rerender()
    const second = result.current
    expect(Object.is(first, second)).toBe(true)
  })

  it('calls client.setBookISBN with bookId and isbn13', async () => {
    const mockSet = jest.fn().mockResolvedValue({})
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce({ setBookISBN: mockSet })
    const { result } = renderHook(() => useSetBookISBN())
    await result.current('book-1', '9780140449112')
    expect(mockSet).toHaveBeenCalledWith({ bookId: 'book-1', isbn13: '9780140449112' })
  })
})

describe('useCreateBook', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useCreateBook())
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
  it('uses /books/kobo/devices as SWR key', () => {
    renderHook(() => useListKoboDevices())
    expect(mockUseSWR).toHaveBeenCalledWith('/books/kobo/devices', expect.any(Function))
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
      ['/books/kepub-status', 'book-abc'],
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
      ['/books/file', 'book-abc', 'pdf'],
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
