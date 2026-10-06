import { act, renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
const mockGetReadingState = jest.fn()
const mockKeepaliveSave = jest.fn()
const mockTranslate = jest.fn()
jest.mock('@/lib/client', () => ({
  createServiceClient: () => ({
    getReadingState: mockGetReadingState,
    translateReadingPosition: mockTranslate
  }),
  createKeepaliveClient: () => ({ updateReadingProgress: mockKeepaliveSave })
}))
jest.mock('@/lib/gen/books/v1/library_pb', () => ({ LibraryService: {} }))
const mockEnqueue = jest.fn()
jest.mock('@/lib/offline/outbox', () => ({
  enqueueWrite: (...args: unknown[]) => mockEnqueue(...args)
}))
jest.mock('@/lib/books/offlineWrites', () => ({ updateReadingProgressWrite: { id: 'progress' } }))

import useSWR from 'swr'
import {
  READING_SAVE_DELAY_MS,
  translateReadingPosition,
  useReadingProgressSaver,
  useReadingState
} from '@/hooks/useReadingState'
import { swrKeys } from '@/lib/swrKeys'
import { updateReadingProgressWrite as mockWrite } from '@/lib/books/offlineWrites'

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

describe('translateReadingPosition', () => {
  it('returns the server-translated position', async () => {
    const translated = { href: 'OEBPS/index.xhtml', offset: 7, page: 3 }
    mockTranslate.mockResolvedValue({ position: translated })
    await expect(translateReadingPosition('book-1', { page: 3 })).resolves.toBe(translated)
    expect(mockTranslate).toHaveBeenCalledWith({ bookId: 'book-1', position: { page: 3 } })
  })
})

describe('useReadingProgressSaver', () => {
  beforeEach(() => {
    jest.useFakeTimers()
    jest.setSystemTime(new Date('2026-10-02T12:00:00Z'))
    mockEnqueue.mockReset()
    mockEnqueue.mockResolvedValue({})
    mockKeepaliveSave.mockReset()
    mockKeepaliveSave.mockResolvedValue({})
  })
  afterEach(() => jest.useRealTimers())

  it('saves the latest position once reading pauses', () => {
    const { result } = renderHook(() => useReadingProgressSaver('book-1'))
    act(() => result.current({ fraction: 0.1, section: 0, position: { href: 'a', offset: 1 } }))
    jest.advanceTimersByTime(1000)
    act(() => result.current(EPUB_LOCATION))
    jest.advanceTimersByTime(READING_SAVE_DELAY_MS - 1)
    expect(mockEnqueue).not.toHaveBeenCalled()

    jest.advanceTimersByTime(1)
    expect(mockEnqueue).toHaveBeenCalledTimes(1)
    expect(mockEnqueue).toHaveBeenCalledWith(mockWrite, {
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
    expect(mockEnqueue.mock.calls[0][1]).toMatchObject({ position: { page: 4 } })
    expect(mockEnqueue.mock.calls[1][1]).toMatchObject({
      percent: 60,
      position: undefined
    })
  })

  it('flushes a pending save when the page is hidden or unloaded', () => {
    const { result } = renderHook(() => useReadingProgressSaver('book-1'))
    act(() => result.current(EPUB_LOCATION))
    window.dispatchEvent(new Event('pagehide'))
    expect(mockEnqueue).toHaveBeenCalledTimes(1)

    act(() => result.current(EPUB_LOCATION))
    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
    document.dispatchEvent(new Event('visibilitychange'))
    expect(mockEnqueue).toHaveBeenCalledTimes(1)
    Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
    document.dispatchEvent(new Event('visibilitychange'))
    expect(mockEnqueue).toHaveBeenCalledTimes(2)

    // Nothing pending: no extra save, and the timer was cleared.
    window.dispatchEvent(new Event('pagehide'))
    jest.advanceTimersByTime(READING_SAVE_DELAY_MS)
    expect(mockEnqueue).toHaveBeenCalledTimes(2)
  })

  it('also sends directly when the page is hidden or unloads, as it may die before the queue stores it', () => {
    const { result, unmount } = renderHook(() => useReadingProgressSaver('book-1'))
    act(() => result.current(EPUB_LOCATION))
    Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
    document.dispatchEvent(new Event('visibilitychange'))
    expect(mockKeepaliveSave).toHaveBeenCalledWith(mockEnqueue.mock.calls[0][1])

    act(() => result.current(EPUB_LOCATION))
    mockKeepaliveSave.mockRejectedValue(new Error('offline'))
    window.dispatchEvent(new Event('pagehide'))
    expect(mockKeepaliveSave).toHaveBeenCalledTimes(2)
    expect(mockKeepaliveSave).toHaveBeenLastCalledWith(mockEnqueue.mock.calls[1][1])

    act(() => result.current(EPUB_LOCATION))
    unmount()
    expect(mockEnqueue).toHaveBeenCalledTimes(3)
    expect(mockKeepaliveSave).toHaveBeenCalledTimes(2)
  })

  it('flushes when the reader closes', () => {
    const { result, unmount } = renderHook(() => useReadingProgressSaver('book-1'))
    act(() => result.current(EPUB_LOCATION))
    unmount()
    expect(mockEnqueue).toHaveBeenCalledTimes(1)
    window.dispatchEvent(new Event('pagehide'))
    expect(mockEnqueue).toHaveBeenCalledTimes(1)
  })

  it('ignores a save the outbox could not queue', async () => {
    mockEnqueue.mockRejectedValue(new Error('quota'))
    const { result } = renderHook(() => useReadingProgressSaver('book-1'))
    act(() => result.current(EPUB_LOCATION))
    await act(async () => {
      jest.advanceTimersByTime(READING_SAVE_DELAY_MS)
    })
    expect(mockEnqueue).toHaveBeenCalledTimes(1)
  })

  it('waits a full pause after a flush before saving again', () => {
    const { result } = renderHook(() => useReadingProgressSaver('book-1'))
    act(() => result.current(EPUB_LOCATION))
    window.dispatchEvent(new Event('pagehide'))
    jest.advanceTimersByTime(1000)
    act(() => result.current(EPUB_LOCATION))
    jest.advanceTimersByTime(READING_SAVE_DELAY_MS - 1)
    expect(mockEnqueue).toHaveBeenCalledTimes(1)
    jest.advanceTimersByTime(1)
    expect(mockEnqueue).toHaveBeenCalledTimes(2)
  })

  it('stops listening for hide and pagehide after unmount', () => {
    const { result, unmount } = renderHook(() => useReadingProgressSaver('book-1'))
    const save = result.current
    unmount()
    act(() => save(EPUB_LOCATION))
    window.dispatchEvent(new Event('pagehide'))
    Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
    document.dispatchEvent(new Event('visibilitychange'))
    expect(mockEnqueue).not.toHaveBeenCalled()
  })

  it('saves for the current book', () => {
    const { result, rerender } = renderHook(({ id }) => useReadingProgressSaver(id), {
      initialProps: { id: null as string | null }
    })
    act(() => result.current(EPUB_LOCATION))
    window.dispatchEvent(new Event('pagehide'))
    expect(mockEnqueue).not.toHaveBeenCalled()

    rerender({ id: 'book-2' })
    act(() => result.current(EPUB_LOCATION))
    window.dispatchEvent(new Event('pagehide'))
    expect(mockEnqueue).toHaveBeenCalledWith(
      mockWrite,
      expect.objectContaining({ bookId: 'book-2' })
    )
  })
})
