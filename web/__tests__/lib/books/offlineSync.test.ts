import { create, type MessageInitShape } from '@bufbuild/protobuf'
import {
  LibraryResponseSchema,
  UserBookSchema,
  type LibraryResponse,
  type UserBook
} from '@/lib/gen/books/v1/library_pb'
import type { BookFileRef } from '@/lib/books/offlineBooks'
import type { StoredBookInfo } from '@/lib/offline/store'
import {
  canPrefetch,
  planOfflineBooks,
  preferredBookFormat,
  syncOfflineBooks
} from '@/lib/books/offlineSync'

const mockDownload = jest.fn<Promise<unknown>, [BookFileRef]>()
const mockDelete = jest.fn()
const mockList = jest.fn()
jest.mock('@/lib/books/offlineBooks', () => ({
  downloadBookFile: (ref: BookFileRef) => mockDownload(ref),
  deleteStoredBook: (...args: unknown[]) => mockDelete(...args),
  listStoredBooks: () => mockList()
}))

const mockSavePage = jest.fn()
jest.mock('@/lib/offline/session', () => ({
  savePageForOffline: (url: string) => mockSavePage(url)
}))

const mockWarm = jest.fn()
jest.mock('@/lib/books/foliate', () => ({
  warmReaderModules: () => mockWarm()
}))

function book(bookId: string, init: MessageInitShape<typeof UserBookSchema> = {}): UserBook {
  return create(UserBookSchema, {
    id: `ub-${bookId}`,
    bookId,
    status: 'currently-reading',
    formats: ['epub'],
    fileVersions: { epub: `${bookId}-v1` },
    ...init
  })
}

function library(init: MessageInitShape<typeof LibraryResponseSchema>): LibraryResponse {
  return create(LibraryResponseSchema, init)
}

function stored(bookId: string, format: string, version: string): StoredBookInfo {
  return { bookId, format, version, size: 10, savedAt: 1 }
}

function setConnection(value: { saveData?: boolean } | undefined) {
  Object.defineProperty(navigator, 'connection', { value, configurable: true })
}

function setOnline(online: boolean) {
  Object.defineProperty(navigator, 'onLine', { value: online, configurable: true })
}

beforeEach(() => {
  jest.clearAllMocks()
  localStorage.clear()
  setOnline(true)
  setConnection(undefined)
  mockList.mockResolvedValue([])
  mockDownload.mockResolvedValue(new Blob(['x']))
  mockDelete.mockResolvedValue(undefined)
})

describe('preferredBookFormat', () => {
  it('reads the original EPUB, else the PDF, by default', () => {
    expect(preferredBookFormat(book('a', { formats: ['pdf', 'epub'] }))).toBe('epub')
    expect(preferredBookFormat(book('b', { formats: ['pdf'] }))).toBe('pdf')
  })

  it('has no format for a book without an EPUB or PDF', () => {
    expect(preferredBookFormat(book('a', { formats: [] }))).toBeNull()
  })
})

describe('planOfflineBooks', () => {
  it('downloads currently-reading books that are missing or stale', () => {
    const plan = planOfflineBooks(
      library({ reading: [book('new'), book('stale'), book('fresh')] }),
      [stored('stale', 'epub', 'old'), stored('fresh', 'epub', 'fresh-v1')]
    )
    expect(plan.downloads).toEqual([
      { bookId: 'new', format: 'epub', version: 'new-v1', readerPath: '/books/ub-new/read' },
      { bookId: 'stale', format: 'epub', version: 'stale-v1', readerPath: '/books/ub-stale/read' }
    ])
    expect(plan.evictions).toEqual([])
  })

  it('skips books with no reader file or no ready original file', () => {
    const plan = planOfflineBooks(
      library({
        reading: [
          book('no-file', { formats: [], fileVersions: {} }),
          book('unknown', { fileVersions: {} })
        ]
      }),
      []
    )
    expect(plan.downloads).toEqual([])
  })

  it('only downloads currently-reading books', () => {
    const plan = planOfflineBooks(
      library({
        wishlist: [book('w', { status: 'to-read' })],
        finished: [book('f', { status: 'read' })]
      }),
      []
    )
    expect(plan.downloads).toEqual([])
  })

  it('evicts finished books and books missing from the library', () => {
    const plan = planOfflineBooks(
      library({
        reading: [book('r')],
        finished: [book('f', { status: 'read' })]
      }),
      [stored('r', 'epub', 'r-v1'), stored('f', 'epub', 'f-v1'), stored('gone', 'pdf', 'x')]
    )
    expect(plan.evictions).toEqual([
      { bookId: 'f', format: 'epub' },
      { bookId: 'gone', format: 'pdf' }
    ])
  })

  it('keeps books opened in the reader that are on other shelves', () => {
    const plan = planOfflineBooks(
      library({
        wishlist: [book('w', { status: 'to-read' })],
        shelves: [{ name: 'dropped', books: [book('d', { status: 'dropped' })] }]
      }),
      [stored('w', 'epub', 'w-v1'), stored('d', 'epub', 'd-v1')]
    )
    expect(plan.evictions).toEqual([])
  })

  it('drops any stored KEPUB, keeping the originals', () => {
    const ub = book('o', { formats: ['epub', 'pdf'], fileVersions: { epub: 'e', pdf: 'p' } })
    const plan = planOfflineBooks(library({ reading: [ub] }), [
      stored('o', 'epub', 'e'),
      stored('o', 'pdf', 'p'),
      stored('o', 'kepub', 'k')
    ])
    expect(plan).toEqual({ downloads: [], evictions: [{ bookId: 'o', format: 'kepub' }] })
  })
})

describe('canPrefetch', () => {
  it('needs a connection without data saving', () => {
    expect(canPrefetch()).toBe(true)
    setConnection({ saveData: true })
    expect(canPrefetch()).toBe(false)
    setConnection({ saveData: false })
    setOnline(false)
    expect(canPrefetch()).toBe(false)
  })
})

describe('syncOfflineBooks', () => {
  it('downloads each missing book once, in order, and saves its reader page', async () => {
    const lib = library({ reading: [book('s1'), book('s2')] })
    await syncOfflineBooks(lib, { gapMs: 0 })

    expect(mockDownload.mock.calls).toEqual([
      [{ bookId: 's1', format: 'epub', version: 's1-v1' }],
      [{ bookId: 's2', format: 'epub', version: 's2-v1' }]
    ])
    expect(mockSavePage.mock.calls).toEqual([['/books/ub-s1/read'], ['/books/ub-s2/read']])

    // Once stored, a later run fetches nothing.
    mockList.mockResolvedValue([stored('s1', 'epub', 's1-v1'), stored('s2', 'epub', 's2-v1')])
    mockDownload.mockClear()
    await syncOfflineBooks(lib, { gapMs: 0 })
    expect(mockDownload).not.toHaveBeenCalled()
  })

  it('never downloads two books at once', async () => {
    let active = 0
    let maxActive = 0
    mockDownload.mockImplementation(async () => {
      maxActive = Math.max(maxActive, ++active)
      await new Promise((r) => setTimeout(r, 1))
      active--
    })
    const lib = library({ reading: [book('p1'), book('p2')] })
    await Promise.all([syncOfflineBooks(lib, { gapMs: 0 }), syncOfflineBooks(lib, { gapMs: 0 })])
    expect(maxActive).toBe(1)
  })

  it('reruns once with the newest library when triggered during a run', async () => {
    let release!: () => void
    mockDownload.mockImplementationOnce(() => new Promise<void>((r) => (release = r)))
    const first = syncOfflineBooks(library({ reading: [book('q1')] }), { gapMs: 0 })
    await new Promise((r) => setTimeout(r, 0))
    const second = syncOfflineBooks(library({ reading: [book('q2')] }), { gapMs: 0 })
    const third = syncOfflineBooks(library({ reading: [book('q3')] }), { gapMs: 0 })
    release()
    await Promise.all([first, second, third])
    expect(mockDownload.mock.calls.map(([ref]) => ref.bookId)).toEqual(['q1', 'q3'])
  })

  it('pauses between downloads', async () => {
    jest.useFakeTimers()
    try {
      const run = syncOfflineBooks(library({ reading: [book('g1'), book('g2')] }), {
        gapMs: 5000
      })
      await jest.advanceTimersByTimeAsync(0)
      expect(mockDownload).toHaveBeenCalledTimes(1)
      await jest.advanceTimersByTimeAsync(4999)
      expect(mockDownload).toHaveBeenCalledTimes(1)
      await jest.advanceTimersByTimeAsync(1)
      expect(mockDownload).toHaveBeenCalledTimes(2)
      await run
    } finally {
      jest.useRealTimers()
    }
  })

  it('skips downloads when data saving is on, but still evicts', async () => {
    setConnection({ saveData: true })
    mockList.mockResolvedValue([stored('gone', 'epub', 'x')])
    await syncOfflineBooks(library({ reading: [book('sd')] }), { gapMs: 0 })
    expect(mockDownload).not.toHaveBeenCalled()
    expect(mockDelete).toHaveBeenCalledWith('gone', 'epub')
  })

  it('skips downloads while offline', async () => {
    setOnline(false)
    await syncOfflineBooks(library({ reading: [book('off')] }), { gapMs: 0 })
    expect(mockDownload).not.toHaveBeenCalled()
  })

  it('stops without backing off when the connection drops during the pause', async () => {
    jest.useFakeTimers()
    try {
      const lib = library({ reading: [book('c1'), book('c2')] })
      const run = syncOfflineBooks(lib, { gapMs: 1000 })
      await jest.advanceTimersByTimeAsync(0)
      setOnline(false)
      await jest.advanceTimersByTimeAsync(1000)
      await run
      expect(mockDownload).toHaveBeenCalledTimes(1)
    } finally {
      jest.useRealTimers()
    }
  })

  it('does not back off a download that failed because the connection dropped', async () => {
    mockDownload.mockImplementationOnce(async () => {
      setOnline(false)
      throw new TypeError('Failed to fetch')
    })
    const lib = library({ reading: [book('nb')] })
    await syncOfflineBooks(lib, { gapMs: 0 })
    setOnline(true)
    await syncOfflineBooks(lib, { gapMs: 0 })
    expect(mockDownload).toHaveBeenCalledTimes(2)
  })

  it('stops when the connection drops mid-run', async () => {
    mockDownload.mockImplementationOnce(async () => setOnline(false))
    await syncOfflineBooks(library({ reading: [book('d1'), book('d2')] }), { gapMs: 0 })
    expect(mockDownload).toHaveBeenCalledTimes(1)
  })

  it('backs off a failed download, then retries it later', async () => {
    const now = jest.spyOn(Date, 'now').mockReturnValue(1_000_000)
    try {
      mockDownload.mockRejectedValueOnce(new Error('404'))
      const lib = library({ reading: [book('bad'), book('ok')] })
      await syncOfflineBooks(lib, { gapMs: 0 })
      expect(mockDownload).toHaveBeenCalledTimes(2)
      expect(mockSavePage).toHaveBeenCalledTimes(1)

      mockList.mockResolvedValue([stored('ok', 'epub', 'ok-v1')])
      mockDownload.mockClear()
      await syncOfflineBooks(lib, { gapMs: 0 })
      expect(mockDownload).not.toHaveBeenCalled()

      now.mockReturnValue(1_000_000 + 29 * 60 * 1000)
      await syncOfflineBooks(lib, { gapMs: 0 })
      expect(mockDownload).not.toHaveBeenCalled()

      now.mockReturnValue(1_000_000 + 30 * 60 * 1000)
      await syncOfflineBooks(lib, { gapMs: 0 })
      expect(mockDownload).toHaveBeenCalledWith({
        bookId: 'bad',
        format: 'epub',
        version: 'bad-v1'
      })
    } finally {
      now.mockRestore()
    }
  })

  it('evicts a stored KEPUB while syncing', async () => {
    const ub = book('sw', { fileVersions: { epub: 'e', kepub: 'k' } })
    mockList.mockResolvedValue([stored('sw', 'epub', 'e'), stored('sw', 'kepub', 'k')])
    await syncOfflineBooks(library({ reading: [ub] }), { gapMs: 0 })
    expect(mockDelete).toHaveBeenCalledWith('sw', 'kepub')
    expect(mockDownload).not.toHaveBeenCalled()
  })

  it('does not warm the reader modules once every stored book is evicted', async () => {
    mockList.mockResolvedValue([stored('gone', 'epub', 'x')])
    await syncOfflineBooks(library({}), { gapMs: 0 })
    expect(mockWarm).not.toHaveBeenCalled()
  })

  it('warms the reader modules while a stored book survives eviction', async () => {
    mockList.mockResolvedValue([stored('gone', 'epub', 'x'), stored('kept', 'epub', 'kept-v1')])
    await syncOfflineBooks(library({ wishlist: [book('kept', { status: 'to-read' })] }), {
      gapMs: 0
    })
    expect(mockWarm).toHaveBeenCalled()
  })

  it('warms the reader modules while any book is stored', async () => {
    await syncOfflineBooks(library({}), { gapMs: 0 })
    expect(mockWarm).not.toHaveBeenCalled()

    mockList.mockResolvedValue([stored('w', 'epub', 'w-v1')])
    await syncOfflineBooks(library({ reading: [book('w')] }), { gapMs: 0 })
    expect(mockWarm).toHaveBeenCalled()
  })
})
