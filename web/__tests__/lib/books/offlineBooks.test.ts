/** @jest-environment node */
import 'fake-indexeddb/auto'
import {
  deleteStoredBook,
  downloadBookFile,
  listStoredBooks,
  openBookFile,
  storedBookIds,
  storedBooksSize,
  subscribeStoredBooks
} from '@/lib/books/offlineBooks'
import { clearStore, loadBookFile, saveBookFile } from '@/lib/offline/store'

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

    await deleteStoredBook('b1', 'epub')
    expect(listener).toHaveBeenCalledTimes(2)
    expect(await listStoredBooks()).toEqual([])

    unsubscribe()
    await downloadBookFile(ref)
    expect(listener).toHaveBeenCalledTimes(2)
  })
})
