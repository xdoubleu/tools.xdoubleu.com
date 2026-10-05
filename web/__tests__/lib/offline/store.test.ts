/** @jest-environment node */
import 'fake-indexeddb/auto'
import {
  addFailed,
  addQueued,
  clearFailed,
  clearStore,
  deleteBookFile,
  deleteQueued,
  getOwner,
  listBookFiles,
  listFailed,
  listQueued,
  loadBookFile,
  loadEntry,
  pruneEntries,
  saveBookFile,
  saveEntry,
  setOwner,
  type StoredBookFile
} from '@/lib/offline/store'

function bookFile(bookId: string, format = 'epub'): StoredBookFile {
  return {
    bookId,
    format,
    version: 'f1:0',
    blob: new Blob(['PK\u0003\u0004'], { type: 'application/epub+zip' }),
    size: 4,
    savedAt: 1
  }
}

describe('offline store', () => {
  beforeEach(async () => {
    await clearStore()
  })

  it('round-trips an entry with its save time', async () => {
    jest.spyOn(Date, 'now').mockReturnValue(1000)
    await saveEntry('k', { n: 1n, bytes: new Uint8Array([1]) })
    jest.restoreAllMocks()

    expect(await loadEntry('k')).toEqual({
      data: { n: 1n, bytes: new Uint8Array([1]) },
      savedAt: 1000
    })
    expect(await loadEntry('missing')).toBeUndefined()
  })

  it('stores the owner and clears it with the entries', async () => {
    await setOwner('u1')
    await saveEntry('k', 1)
    expect(await getOwner()).toBe('u1')

    await clearStore()

    expect(await getOwner()).toBeUndefined()
    expect(await loadEntry('k')).toBeUndefined()
  })

  it('prunes only entries saved before the cutoff', async () => {
    const now = jest.spyOn(Date, 'now')
    now.mockReturnValue(100)
    await saveEntry('old', 1)
    now.mockReturnValue(300)
    await saveEntry('new', 2)
    now.mockRestore()

    await pruneEntries(200)

    expect(await loadEntry('old')).toBeUndefined()
    expect(await loadEntry('new')).toEqual({ data: 2, savedAt: 300 })
  })
})

describe('offline store queue', () => {
  beforeEach(async () => {
    await clearStore()
  })

  const write = (writeId: string) => ({
    writeId,
    request: new Uint8Array([1, 2]),
    hint: { a: 'b' },
    createdAt: 1
  })

  it('keeps queued writes in insertion order until deleted', async () => {
    const first = await addQueued(write('w/A'))
    const second = await addQueued(write('w/B'))
    expect(second).toBeGreaterThan(first ?? Infinity)

    expect((await listQueued()).map((w) => w.writeId)).toEqual(['w/A', 'w/B'])

    await deleteQueued(first ?? 0)
    expect(await listQueued()).toEqual([{ ...write('w/B'), seq: second }])
  })

  it('stores failed writes until cleared, and clearStore wipes both', async () => {
    await addFailed({ description: 'Add “milk”', reason: 'gone' })
    expect(await listFailed()).toEqual([
      { description: 'Add “milk”', reason: 'gone', seq: expect.any(Number) }
    ])

    await clearFailed()
    expect(await listFailed()).toEqual([])

    await addQueued(write('w/A'))
    await addFailed({ description: 'x', reason: 'y' })
    await clearStore()
    expect(await listQueued()).toEqual([])
    expect(await listFailed()).toEqual([])
  })
})

describe('offline store write failures', () => {
  it('swallows values IndexedDB cannot store', async () => {
    await expect(saveEntry('fn', () => {})).resolves.toBeUndefined()
    expect(await loadEntry('fn')).toBeUndefined()
  })
})

describe('offline store without IndexedDB', () => {
  it('resolves to undefined and no-ops', async () => {
    const original = globalThis.indexedDB
    // @ts-expect-error -- simulate a browser without IndexedDB
    delete globalThis.indexedDB
    try {
      await saveEntry('k', 1)
      expect(await loadEntry('k')).toBeUndefined()
      expect(await getOwner()).toBeUndefined()
      expect(
        await addQueued({ writeId: 'w', request: new Uint8Array(), createdAt: 1 })
      ).toBeUndefined()
      expect(await listQueued()).toEqual([])
      expect(await listFailed()).toEqual([])
    } finally {
      globalThis.indexedDB = original
    }
  })

  it('stores book files by book and format, listing them without the bytes', async () => {
    await saveBookFile(bookFile('b1'))
    await saveBookFile(bookFile('b1', 'pdf'))
    await saveBookFile({ ...bookFile('b2'), version: 'f2:0' })

    const stored = await loadBookFile('b1', 'epub')
    expect(stored).toMatchObject({ bookId: 'b1', format: 'epub', version: 'f1:0', size: 4 })
    expect(await stored?.blob.text()).toBe('PK\u0003\u0004')
    expect(await loadBookFile('b1', 'kepub')).toBeUndefined()

    const listed = await listBookFiles()
    expect(listed).toHaveLength(3)
    expect(listed).toContainEqual({
      bookId: 'b2',
      format: 'epub',
      version: 'f2:0',
      size: 4,
      savedAt: 1
    })

    await deleteBookFile('b1', 'pdf')
    expect(await loadBookFile('b1', 'pdf')).toBeUndefined()
    expect(await loadBookFile('b1', 'epub')).toBeDefined()
  })

  it('wipes stored books on clear but never prunes them', async () => {
    await saveBookFile(bookFile('b1'))

    await pruneEntries(Number.MAX_SAFE_INTEGER)
    expect(await loadBookFile('b1', 'epub')).toBeDefined()

    await clearStore()
    expect(await listBookFiles()).toEqual([])
  })
})
