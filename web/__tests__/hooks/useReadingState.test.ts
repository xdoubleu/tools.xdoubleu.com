import { act, renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
const mockUpdateReadingProgress = jest.fn()
const mockGetReadingState = jest.fn()
jest.mock('@/lib/client', () => ({
  createServiceClient: () => ({
    updateReadingProgress: mockUpdateReadingProgress,
    getReadingState: mockGetReadingState
  })
}))
jest.mock('@/lib/gen/books/v1/library_pb', () => ({ LibraryService: {} }))

import useSWR from 'swr'
import {
  READING_SAVE_DELAY_MS,
  useReadingProgressSaver,
  useReadingState
} from '@/hooks/useReadingState'
import { swrKeys } from '@/lib/swrKeys'

const mockUseSWR = jest.mocked(useSWR)

const EPUB_LOCATION = {
  fraction: 0.426,
  section: 1,
  position: { href: 'OEBPS/ch1.xhtml', offset: 120 }
}

describe('useReadingState', () => {
  beforeEach(() => mockUseSWR.mockReset())

  it('fetches the book reading state', async () => {
    renderHook(() => useReadingState('book-1'))
    expect(mockUseSWR.mock.calls[0]![0]).toEqual(swrKeys.readingState('book-1'))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    mockGetReadingState.mockResolvedValue({ state: { percent: 3 } })
    await expect(fetcher()).resolves.toEqual({ state: { percent: 3 } })
    expect(mockGetReadingState).toHaveBeenCalledWith({ bookId: 'book-1' })
  })

  it('waits for a book id', () => {
    renderHook(() => useReadingState(null))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })
})

describe('useReadingProgressSaver', () => {
  beforeEach(() => {
    jest.useFakeTimers()
    jest.setSystemTime(new Date('2026-10-02T12:00:00Z'))
    mockUpdateReadingProgress.mockReset()
    mockUpdateReadingProgress.mockResolvedValue({})
  })
  afterEach(() => jest.useRealTimers())

  it('saves the latest position once reading pauses', () => {
    const { result } = renderHook(() => useReadingProgressSaver('book-1'))
    act(() => result.current({ fraction: 0.1, section: 0, position: { href: 'a', offset: 1 } }))
    jest.advanceTimersByTime(1000)
    act(() => result.current(EPUB_LOCATION))
    jest.advanceTimersByTime(READING_SAVE_DELAY_MS - 1)
    expect(mockUpdateReadingProgress).not.toHaveBeenCalled()

    jest.advanceTimersByTime(1)
    expect(mockUpdateReadingProgress).toHaveBeenCalledTimes(1)
    expect(mockUpdateReadingProgress).toHaveBeenCalledWith({
      bookId: 'book-1',
      source: 'web',
      percent: 43,
      position: { href: 'OEBPS/ch1.xhtml', offset: 120 },
      readAt: '2026-10-02T12:00:01.000Z'
    })
  })

  it('sends a PDF page and saves percent alone without a position', () => {
    const { result } = renderHook(() => useReadingProgressSaver('book-1'))
    act(() => result.current({ fraction: 0.5, section: 3, position: { page: 4 } }))
    jest.advanceTimersByTime(READING_SAVE_DELAY_MS)
    act(() => result.current({ fraction: 0.6, section: 4 }))
    jest.advanceTimersByTime(READING_SAVE_DELAY_MS)
    expect(mockUpdateReadingProgress.mock.calls[0][0]).toMatchObject({ position: { page: 4 } })
    expect(mockUpdateReadingProgress.mock.calls[1][0]).toMatchObject({
      percent: 60,
      position: undefined
    })
  })

  it('flushes a pending save when the page is hidden or unloaded', () => {
    const { result } = renderHook(() => useReadingProgressSaver('book-1'))
    act(() => result.current(EPUB_LOCATION))
    window.dispatchEvent(new Event('pagehide'))
    expect(mockUpdateReadingProgress).toHaveBeenCalledTimes(1)

    act(() => result.current(EPUB_LOCATION))
    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
    document.dispatchEvent(new Event('visibilitychange'))
    expect(mockUpdateReadingProgress).toHaveBeenCalledTimes(1)
    Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
    document.dispatchEvent(new Event('visibilitychange'))
    expect(mockUpdateReadingProgress).toHaveBeenCalledTimes(2)

    // Nothing pending: no extra save, and the timer was cleared.
    window.dispatchEvent(new Event('pagehide'))
    jest.advanceTimersByTime(READING_SAVE_DELAY_MS)
    expect(mockUpdateReadingProgress).toHaveBeenCalledTimes(2)
  })

  it('flushes when the reader closes', () => {
    const { result, unmount } = renderHook(() => useReadingProgressSaver('book-1'))
    act(() => result.current(EPUB_LOCATION))
    unmount()
    expect(mockUpdateReadingProgress).toHaveBeenCalledTimes(1)
    window.dispatchEvent(new Event('pagehide'))
    expect(mockUpdateReadingProgress).toHaveBeenCalledTimes(1)
  })

  it('ignores a failed save', async () => {
    mockUpdateReadingProgress.mockRejectedValue(new Error('offline'))
    const { result } = renderHook(() => useReadingProgressSaver('book-1'))
    act(() => result.current(EPUB_LOCATION))
    await act(async () => {
      jest.advanceTimersByTime(READING_SAVE_DELAY_MS)
    })
    expect(mockUpdateReadingProgress).toHaveBeenCalledTimes(1)
  })

  it('waits a full pause after a flush before saving again', () => {
    const { result } = renderHook(() => useReadingProgressSaver('book-1'))
    act(() => result.current(EPUB_LOCATION))
    window.dispatchEvent(new Event('pagehide'))
    jest.advanceTimersByTime(1000)
    act(() => result.current(EPUB_LOCATION))
    jest.advanceTimersByTime(READING_SAVE_DELAY_MS - 1)
    expect(mockUpdateReadingProgress).toHaveBeenCalledTimes(1)
    jest.advanceTimersByTime(1)
    expect(mockUpdateReadingProgress).toHaveBeenCalledTimes(2)
  })

  it('stops listening for hide and pagehide after unmount', () => {
    const { result, unmount } = renderHook(() => useReadingProgressSaver('book-1'))
    const save = result.current
    unmount()
    act(() => save(EPUB_LOCATION))
    window.dispatchEvent(new Event('pagehide'))
    Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
    document.dispatchEvent(new Event('visibilitychange'))
    expect(mockUpdateReadingProgress).not.toHaveBeenCalled()
  })

  it('saves for the current book', () => {
    const { result, rerender } = renderHook(({ id }) => useReadingProgressSaver(id), {
      initialProps: { id: null as string | null }
    })
    act(() => result.current(EPUB_LOCATION))
    window.dispatchEvent(new Event('pagehide'))
    expect(mockUpdateReadingProgress).not.toHaveBeenCalled()

    rerender({ id: 'book-2' })
    act(() => result.current(EPUB_LOCATION))
    window.dispatchEvent(new Event('pagehide'))
    expect(mockUpdateReadingProgress).toHaveBeenCalledWith(
      expect.objectContaining({ bookId: 'book-2' })
    )
  })
})
