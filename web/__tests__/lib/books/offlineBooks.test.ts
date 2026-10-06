/** @jest-environment node */
import 'fake-indexeddb/auto'
import {
  deleteAllStoredBooks,
  deleteStoredBook,
  downloadBookFile,
  listStoredBooks,
  openBookFile,
  storedBookIds,
  storedBookVersion,
  storedBooksSize,
  subscribeStoredBooks
} from '@/lib/books/offlineBooks'
import { clearStore, loadBookFile, loadEntry, saveBookFile, saveEntry } from '@/lib/offline/store'

const mockGetBookFile = jest.fn()
jest.mock('@/lib/client', () => ({
  createServiceClient: () => ({ getBookFile: mockGetBookFile })
}))

const fetchMock = jest.spyOn(globalThis, 'fetch')

const ref = { bookId: 'b1', format: 'epub', version: 'f1:0' }

function serve(body: string) {
  mockGetBookFile.mockResolvedValue({ url: 'https://r2/signed', format: 'epub' })
  fetchMock.mockImplementation(
    async () => new Response(body, { headers: { 'Content-Type': 'application/epub+zip' } })
  )
}

async function store(version: string, body = 'stored') {
  await saveBookFile({
    bookId: 'b1',
    format: 'epub',
    version,
    blob: new Blob([body]),
    size: body.length,
    savedAt: 1
  })
}

beforeEach(async () => {
  jest.clearAllMocks()
  await clearStore()
})

describe('openBookFile', () => {
  it('opens the stored copy without touching the network', async () => {
    await store('f1:0')

    const file = await openBookFile(ref)

    expect(await file.text()).toBe('stored')
    expect(file.name).toBe('b1.epub')
    expect(mockGetBookFile).not.toHaveBeenCalled()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('opens a download without waiting for it to be stored, then stores it', async () => {
    serve('fresh')
    const stored = new Promise<void>((resolve) => {
      const unsubscribe = subscribeStoredBooks(() => {
        unsubscribe()
        resolve()
      })
    })

    const file = await openBookFile(ref)

    expect(await file.text()).toBe('fresh')
    expect(mockGetBookFile).toHaveBeenCalledWith({ bookId: 'b1', format: 'epub' })
    expect(fetchMock).toHaveBeenCalledWith('https://r2/signed')
    await stored
    expect(await loadBookFile('b1', 'epub')).toMatchObject({ version: 'f1:0', size: 5 })
  })

  it('refetches a stored copy whose version is stale', async () => {
    await store('f0:0')
    serve('newer')

    const file = await openBookFile(ref)

    expect(await file.text()).toBe('newer')
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect((await loadBookFile('b1', 'epub'))?.version).toBe('f1:0')
  })

  it('keeps reading a stale copy when the refetch fails', async () => {
    await store('f0:0')
    mockGetBookFile.mockRejectedValue(new TypeError('Failed to fetch'))

    const file = await openBookFile(ref)

    expect(await file.text()).toBe('stored')
  })

  it('fails when nothing is stored and the download fails', async () => {
    mockGetBookFile.mockResolvedValue({ url: 'https://r2/signed' })
    fetchMock.mockResolvedValue(new Response('', { status: 403 }))

    await expect(openBookFile(ref)).rejects.toThrow('403')
    expect(await loadBookFile('b1', 'epub')).toBeUndefined()
  })
})

describe('a wipe during a download', () => {
  // Resolves once the file's fetch is in flight.
  async function fetchStarted() {
    while (fetchMock.mock.calls.length === 0) await new Promise((resolve) => setTimeout(resolve, 1))
  }

  it('drops a file downloaded for the wiped session', async () => {
    let finish: ((res: Response) => void) | undefined
    mockGetBookFile.mockResolvedValue({ url: 'https://r2/signed' })
    fetchMock.mockImplementation(() => new Promise<Response>((resolve) => (finish = resolve)))

    const download = downloadBookFile(ref)
    await fetchStarted()
    await clearStore()
    finish?.(new Response('late'))

    expect(await (await download).text()).toBe('late')
    expect(await loadBookFile('b1', 'epub')).toBeUndefined()
  })

  it('does not store a file opened before the wipe', async () => {
    let finish: ((res: Response) => void) | undefined
    mockGetBookFile.mockResolvedValue({ url: 'https://r2/signed' })
    fetchMock.mockImplementation(() => new Promise<Response>((resolve) => (finish = resolve)))

    const open = openBookFile(ref)
    await fetchStarted()
    await clearStore()
    finish?.(new Response('late'))

    expect(await (await open).text()).toBe('late')
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(await loadBookFile('b1', 'epub')).toBeUndefined()
  })
})

describe('stored book management', () => {
  it('lists, sizes and deletes stored books, notifying subscribers', async () => {
    const listener = jest.fn()
    const unsubscribe = subscribeStoredBooks(listener)
    serve('12345')

    await downloadBookFile(ref)
    expect(listener).toHaveBeenCalledTimes(1)
    expect(await listStoredBooks()).toEqual([
      expect.objectContaining({ bookId: 'b1', format: 'epub', size: 5 })
    ])
    expect(await storedBooksSize()).toBe(5)
    expect(await storedBookIds()).toEqual(new Set(['b1']))
    expect(await storedBookVersion('b1', 'epub')).toBe('f1:0')
    expect(await storedBookVersion('b1', 'kepub')).toBeNull()

    await deleteStoredBook('b1', 'epub')
    expect(listener).toHaveBeenCalledTimes(2)
    expect(await listStoredBooks()).toEqual([])

    unsubscribe()
    await downloadBookFile(ref)
    expect(listener).toHaveBeenCalledTimes(2)
  })

  it('deletes every stored book, and nothing else', async () => {
    await store('f1:0')
    await saveBookFile({
      bookId: 'b2',
      format: 'pdf',
      version: 'p',
      blob: new Blob(['p']),
      size: 1,
      savedAt: 1
    })
    await saveEntry('library', { kept: true })
    const listener = jest.fn()
    const unsubscribe = subscribeStoredBooks(listener)

    await deleteAllStoredBooks()

    expect(await listStoredBooks()).toEqual([])
    expect((await loadEntry('library'))?.data).toEqual({ kept: true })
    expect(listener).toHaveBeenCalledTimes(1)
    unsubscribe()
  })
})
