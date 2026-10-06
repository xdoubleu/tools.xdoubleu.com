import { act, renderHook, waitFor } from '@testing-library/react'
import { useOfflineBookFile, useStoredBookIds, useStoredBookVersion } from '@/hooks/useOfflineBooks'

const mockOpenBookFile = jest.fn()
const mockStoredBookIds = jest.fn()
const mockStoredBookVersion = jest.fn()
const storedListeners = new Set<() => void>()

jest.mock('@/lib/books/offlineBooks', () => ({
  openBookFile: (...args: unknown[]) => mockOpenBookFile(...args),
  storedBookIds: () => mockStoredBookIds(),
  storedBookVersion: (...args: unknown[]) => mockStoredBookVersion(...args),
  subscribeStoredBooks: (listener: () => void) => {
    storedListeners.add(listener)
    return () => storedListeners.delete(listener)
  }
}))

beforeEach(() => {
  jest.clearAllMocks()
  mockStoredBookIds.mockResolvedValue(new Set())
})

describe('useOfflineBookFile', () => {
  it('opens the file once, ignoring later version changes', async () => {
    const file = new File(['x'], 'b1.epub')
    mockOpenBookFile.mockResolvedValue(file)

    const { result, rerender } = renderHook(
      ({ version }) => useOfflineBookFile('b1', 'epub', version),
      { initialProps: { version: 'v1' } }
    )
    expect(result.current).toEqual({})
    await waitFor(() => expect(result.current.file).toBe(file))

    rerender({ version: 'v2' })
    expect(mockOpenBookFile).toHaveBeenCalledTimes(1)
    expect(mockOpenBookFile).toHaveBeenCalledWith({ bookId: 'b1', format: 'epub', version: 'v1' })
  })

  it('reports a failed open', async () => {
    const error = new Error('offline')
    mockOpenBookFile.mockRejectedValue(error)

    const { result } = renderHook(() => useOfflineBookFile('b1', 'pdf', ''))

    await waitFor(() => expect(result.current.error).toBe(error))
    expect(result.current.file).toBeUndefined()
  })

  it('retries a failed open when the connection returns', async () => {
    const file = new File(['x'], 'b1.epub')
    mockOpenBookFile.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(file)

    const { result, unmount } = renderHook(() => useOfflineBookFile('b1', 'epub', ''))
    await waitFor(() => expect(result.current.error).toBeDefined())

    act(() => void window.dispatchEvent(new Event('online')))
    await waitFor(() => expect(result.current.file).toBe(file))
    expect(mockOpenBookFile).toHaveBeenCalledTimes(2)

    // Once open, reconnecting doesn't reopen it.
    act(() => void window.dispatchEvent(new Event('online')))
    unmount()
    expect(mockOpenBookFile).toHaveBeenCalledTimes(2)
  })

  it('does nothing without a book or format', () => {
    const { result } = renderHook(() => useOfflineBookFile('b1', null, ''))
    expect(result.current).toEqual({})
    expect(mockOpenBookFile).not.toHaveBeenCalled()
  })

  it('drops the previous file when the format changes', async () => {
    let resolvePdf: (f: File) => void = () => {}
    const epub = new File(['e'], 'b1.epub')
    mockOpenBookFile
      .mockResolvedValueOnce(epub)
      .mockReturnValueOnce(new Promise((r) => (resolvePdf = r)))

    const { result, rerender, unmount } = renderHook(
      ({ format }) => useOfflineBookFile('b1', format, ''),
      { initialProps: { format: 'epub' } }
    )
    await waitFor(() => expect(result.current.file).toBe(epub))

    rerender({ format: 'pdf' })
    expect(result.current).toEqual({})
    unmount()
    // Landing after unmount is ignored.
    await act(async () => resolvePdf(new File(['p'], 'b1.pdf')))
  })
})

describe('useStoredBookIds', () => {
  it('shares one listing across components and refreshes when books change', async () => {
    mockStoredBookIds.mockResolvedValueOnce(new Set(['b1']))
    const first = renderHook(() => useStoredBookIds())
    const second = renderHook(() => useStoredBookIds())
    await waitFor(() => expect([...first.result.current]).toEqual(['b1']))
    expect([...second.result.current]).toEqual(['b1'])
    expect(mockStoredBookIds).toHaveBeenCalledTimes(1)

    mockStoredBookIds.mockResolvedValueOnce(new Set(['b2']))
    act(() => storedListeners.forEach((l) => l()))
    await waitFor(() => expect([...first.result.current]).toEqual(['b2']))

    first.unmount()
    second.unmount()
  })
})

describe('useStoredBookVersion', () => {
  it('reports the stored version, refreshing when books change', async () => {
    mockStoredBookVersion.mockResolvedValueOnce(null)
    const { result, unmount } = renderHook(() => useStoredBookVersion('b1', 'kepub'))
    expect(result.current).toBeUndefined()
    await waitFor(() => expect(result.current).toBeNull())
    expect(mockStoredBookVersion).toHaveBeenCalledWith('b1', 'kepub')

    mockStoredBookVersion.mockResolvedValueOnce('v1')
    act(() => storedListeners.forEach((l) => l()))
    await waitFor(() => expect(result.current).toBe('v1'))
    unmount()
  })

  it('checks nothing without a book', () => {
    const { result } = renderHook(() => useStoredBookVersion(null, 'kepub'))
    expect(result.current).toBeNull()
    expect(mockStoredBookVersion).not.toHaveBeenCalled()
  })
})
