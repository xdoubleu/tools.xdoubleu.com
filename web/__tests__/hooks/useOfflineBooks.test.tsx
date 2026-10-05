import { act, renderHook, waitFor } from '@testing-library/react'
import { useOfflineBookFile, useStoredBookIds } from '@/hooks/useOfflineBooks'

const mockOpenBookFile = jest.fn()
const mockListStoredBooks = jest.fn()
const storedListeners = new Set<() => void>()

jest.mock('@/lib/books/offlineBooks', () => ({
  openBookFile: (...args: unknown[]) => mockOpenBookFile(...args),
  listStoredBooks: () => mockListStoredBooks(),
  subscribeStoredBooks: (listener: () => void) => {
    storedListeners.add(listener)
    return () => storedListeners.delete(listener)
  }
}))

beforeEach(() => {
  jest.clearAllMocks()
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
    mockListStoredBooks.mockResolvedValueOnce([
      { bookId: 'b1', format: 'epub' },
      { bookId: 'b1', format: 'pdf' }
    ])
    const first = renderHook(() => useStoredBookIds())
    const second = renderHook(() => useStoredBookIds())
    await waitFor(() => expect([...first.result.current]).toEqual(['b1']))
    expect([...second.result.current]).toEqual(['b1'])
    expect(mockListStoredBooks).toHaveBeenCalledTimes(1)

    mockListStoredBooks.mockResolvedValueOnce([{ bookId: 'b2', format: 'epub' }])
    act(() => storedListeners.forEach((l) => l()))
    await waitFor(() => expect([...first.result.current]).toEqual(['b2']))

    first.unmount()
    second.unmount()
  })
})
