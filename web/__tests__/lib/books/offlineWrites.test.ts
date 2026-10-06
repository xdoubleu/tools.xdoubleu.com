import { create, isMessage, type DescMessage, type MessageInitShape } from '@bufbuild/protobuf'
import type { OfflineWrite } from '@/lib/offline/registry'
import { keepaliveTransport, transport } from '@/lib/client'
import { swrKeys } from '@/lib/swrKeys'
import {
  GetLibraryResponseSchema,
  GetReadingStateResponseSchema,
  type LibraryResponse
} from '@/lib/gen/books/v1/library_pb'
import {
  bookWrites,
  removeBookWrite,
  setBookISBNWrite,
  setBookTagWrite,
  updateBookStatusWrite,
  updateBookWrite,
  updateFinishedAtWrite,
  updateProgressWrite,
  updateReadingProgressWrite
} from '@/lib/books/offlineWrites'

jest.mock('@/lib/client', () => ({
  transport: { unary: jest.fn(async () => ({})) },
  keepaliveTransport: { unary: jest.fn(async () => ({})) }
}))

function run<I extends DescMessage>(
  write: OfflineWrite<I>,
  init: MessageInitShape<I>,
  data: unknown,
  hint?: unknown,
  key: unknown = swrKeys.books
) {
  return write.apply(key, data, write.encode(init), hint)
}

const NOW = '2024-05-01T10:00:00.000Z'

const book = (bookId: string, title: string, status: string, extra = {}) => ({
  id: `ub-${bookId}`,
  bookId,
  status,
  book: { id: bookId, title, coverUrl: `/covers/${bookId}` },
  ...extra
})

const library = () =>
  create(GetLibraryResponseSchema, {
    library: {
      reading: [
        book('b', 'Beta', 'currently-reading', { tags: ['favourite', 'sci-fi'], rating: 3 })
      ],
      wishlist: [book('a', 'Alpha', 'to-read'), book('c', 'Gamma', 'to-read')],
      finished: [book('d', 'Delta', 'read', { finishedAt: ['2023-01-01T00:00:00Z'] })],
      shelves: [
        { name: 'dropped', books: [] },
        { name: 'zines', books: [book('e', 'Echo', 'zines')] }
      ]
    }
  })

const lib = (data: unknown): LibraryResponse => {
  if (!isMessage(data, GetLibraryResponseSchema) || !data.library) throw new Error('no library')
  return data.library
}
const ids = (list: { bookId: string }[]) => list.map((b) => b.bookId)

describe('book offline writes', () => {
  it('leave unrelated keys, data and unknown books untouched', () => {
    const data = library()
    for (const write of bookWrites) {
      expect(write.apply('/recipes', data, new Uint8Array(), undefined)).toBe(data)
    }
    expect(run(setBookTagWrite, { bookId: 'b', tag: 'x', enabled: true }, 'nope')).toBe('nope')
    expect(run(setBookTagWrite, { bookId: 'zz', tag: 'x', enabled: true }, data)).toBe(data)
    const empty = create(GetLibraryResponseSchema, {})
    expect(run(removeBookWrite, { bookId: 'b' }, empty)).toBe(empty)
    expect(run(setBookTagWrite, { bookId: 'b' }, empty)).toBe(empty)
  })

  it('moves a finished book to the finished list in title order, logging the finish', () => {
    const result = lib(
      run(
        updateBookStatusWrite,
        { bookId: 'b', status: 'read', favourite: false, rating: '5' },
        library(),
        NOW
      )
    )
    expect(ids(result.reading)).toEqual([])
    expect(ids(result.finished)).toEqual(['b', 'd'])
    expect(result.finished[0]).toMatchObject({
      status: 'read',
      tags: ['sci-fi'],
      rating: 5,
      finishedAt: [NOW]
    })
  })

  it('keeps read dates on a re-save as read and clears them when leaving read', () => {
    const again = lib(run(updateBookStatusWrite, { bookId: 'd', status: 'read' }, library(), NOW))
    expect(again.finished[0].finishedAt).toEqual(['2023-01-01T00:00:00Z'])

    const reread = lib(
      run(updateBookStatusWrite, { bookId: 'd', status: 'currently-reading' }, library(), NOW)
    )
    expect(ids(reread.reading)).toEqual(['b', 'd'])
    expect(reread.reading[1].finishedAt).toEqual([])

    const noHint = lib(run(updateBookStatusWrite, { bookId: 'a', status: 'read' }, library()))
    expect(noHint.finished[0].finishedAt).toEqual([''])
  })

  it('keeps the rating unless a valid one is given and adds the favourite tag last', () => {
    for (const rating of ['', '0', '9', 'x']) {
      const result = lib(
        run(
          updateBookStatusWrite,
          { bookId: 'b', status: 'currently-reading', favourite: true, rating },
          library()
        )
      )
      expect(result.reading[0]).toMatchObject({ rating: 3, tags: ['sci-fi', 'favourite'] })
    }
  })

  it('files custom statuses under shelves, creating them in name order', () => {
    const wishlist = lib(run(updateBookStatusWrite, { bookId: 'c', status: 'to-read' }, library()))
    expect(ids(wishlist.wishlist)).toEqual(['a', 'c'])

    const dropped = lib(run(updateBookStatusWrite, { bookId: 'a', status: 'dropped' }, library()))
    expect(ids(dropped.wishlist)).toEqual(['c'])
    expect(dropped.shelves.map((s) => [s.name, ids(s.books)])).toEqual([
      ['dropped', ['a']],
      ['zines', ['e']]
    ])

    const created = lib(run(updateBookStatusWrite, { bookId: 'a', status: 'comics' }, library()))
    expect(created.shelves.map((s) => [s.name, ids(s.books)])).toEqual([
      ['comics', ['a']],
      ['dropped', []],
      ['zines', ['e']]
    ])
    const last = lib(run(updateBookStatusWrite, { bookId: 'a', status: 'zzz' }, library()))
    expect(last.shelves.map((s) => s.name)).toEqual(['dropped', 'zines', 'zzz'])
  })

  it('inserts between titles, ignores other keys and keeps the server order on in-place edits', () => {
    const middle = lib(run(updateBookStatusWrite, { bookId: 'b', status: 'to-read' }, library()))
    expect(ids(middle.wishlist)).toEqual(['a', 'b', 'c'])

    const data = library()
    expect(
      run(updateBookStatusWrite, { bookId: 'b', status: 'read' }, data, NOW, '/books/progress')
    ).toBe(data)

    // The API's collation can order titles differently from localeCompare.
    const unsorted = create(GetLibraryResponseSchema, {
      library: { wishlist: [book('c', 'gamma', 'to-read'), book('a', 'Alpha', 'to-read')] }
    })
    const tagged = lib(run(setBookTagWrite, { bookId: 'c', tag: 'x', enabled: true }, unsorted))
    expect(ids(tagged.wishlist)).toEqual(['c', 'a'])
  })

  it('replaces read dates with the given days', () => {
    const result = lib(
      run(updateFinishedAtWrite, { bookId: 'd', finishedAt: ['2024-02-03', ''] }, library())
    )
    expect(result.finished[0].finishedAt).toEqual(['2024-02-03T00:00:00Z'])
  })

  it('updates progress within bounds and ignores invalid modes', () => {
    const pages = lib(
      run(
        updateProgressWrite,
        { bookId: 'b', progressMode: 'pages', currentPage: -3, progressPercent: 140 },
        library()
      )
    )
    expect(pages.reading[0]).toMatchObject({
      progressMode: 'pages',
      currentPage: 0,
      progressPercent: 100
    })
    const percent = lib(
      run(
        updateProgressWrite,
        { bookId: 'b', progressMode: 'percent', progressPercent: -1 },
        library()
      )
    )
    expect(percent.reading[0]).toMatchObject({ progressMode: 'percent', progressPercent: 0 })

    const data = library()
    expect(run(updateProgressWrite, { bookId: 'b', progressMode: 'chapters' }, data)).toBe(data)
  })

  it('sets and clears a tag in place', () => {
    const added = lib(
      run(setBookTagWrite, { bookId: 'e', tag: 'kobo-sync', enabled: true }, library())
    )
    expect(added.shelves[1].books[0].tags).toEqual(['kobo-sync'])
    const again = lib(
      run(setBookTagWrite, { bookId: 'b', tag: 'sci-fi', enabled: true }, library())
    )
    expect(again.reading[0].tags).toEqual(['favourite', 'sci-fi'])
    const removed = lib(
      run(setBookTagWrite, { bookId: 'b', tag: 'favourite', enabled: false }, library())
    )
    expect(removed.reading[0].tags).toEqual(['sci-fi'])
    expect(ids(removed.wishlist)).toEqual(['a', 'c'])
  })

  it('removes a book from every list', () => {
    const result = lib(run(removeBookWrite, { bookId: 'e' }, library()))
    expect(result.shelves.map((s) => ids(s.books))).toEqual([[], []])
    expect(ids(lib(run(removeBookWrite, { bookId: 'a' }, library())).wishlist)).toEqual(['c'])
  })

  it('normalizes a set ISBN', () => {
    const result = lib(
      run(setBookISBNWrite, { bookId: 'a', isbn13: '978-0 14 0449112' }, library())
    )
    expect(result.wishlist[0].book?.isbn13).toBe('9780140449112')
  })

  it('applies edited metadata, re-sorting on a new title and keeping the cover until fetched', () => {
    const metadata = {
      title: 'Zulu',
      authors: ['Ann'],
      isbn13: '978-0140449112',
      description: 'New',
      pageCount: 120,
      coverUrl: 'https://example.com/new.jpg'
    }
    const result = lib(run(updateBookWrite, { bookId: 'a', metadata }, library()))
    expect(ids(result.wishlist)).toEqual(['c', 'a'])
    expect(result.wishlist[1].book).toMatchObject({
      title: 'Zulu',
      authors: ['Ann'],
      isbn13: '9780140449112',
      description: 'New',
      pageCount: 120,
      coverUrl: '/covers/a'
    })

    const cleared = lib(
      run(updateBookWrite, { bookId: 'a', metadata: { title: 'Alpha', coverUrl: '' } }, library())
    )
    expect(cleared.wishlist[0].book?.coverUrl).toBe('')

    const data = library()
    expect(lib(run(updateBookWrite, { bookId: 'a' }, data)).wishlist[0]).toBe(
      data.library?.wishlist[0]
    )
  })

  it('leaves books without catalog data alone', () => {
    const data = create(GetLibraryResponseSchema, {
      library: { wishlist: [{ bookId: 'x', status: 'to-read' }] }
    })
    expect(lib(run(setBookISBNWrite, { bookId: 'x', isbn13: '1' }, data)).wishlist[0].book).toBe(
      undefined
    )
    expect(
      lib(run(updateBookWrite, { bookId: 'x', metadata: { title: 'T' } }, data)).wishlist[0].book
    ).toBe(undefined)
  })

  it('describes each write', () => {
    const describe = <I extends DescMessage>(w: OfflineWrite<I>, init: MessageInitShape<I>) =>
      w.describe(w.encode(init))
    expect(describe(updateBookStatusWrite, {})).toBe('Update a book’s status')
    expect(describe(updateFinishedAtWrite, {})).toBe('Edit a book’s read dates')
    expect(describe(updateProgressWrite, {})).toBe('Update reading progress')
    expect(describe(setBookTagWrite, { tag: 'x', enabled: true })).toBe('Add the “x” tag')
    expect(describe(setBookTagWrite, { tag: 'x' })).toBe('Remove the “x” tag')
    expect(describe(removeBookWrite, {})).toBe('Remove a book')
    expect(describe(setBookISBNWrite, {})).toBe('Set a book’s ISBN')
    expect(describe(updateBookWrite, { metadata: { title: 'Dune' } })).toBe('Edit “Dune”')
    expect(describe(updateBookWrite, {})).toBe('Edit “a book”')
    expect(describe(updateReadingProgressWrite, {})).toBe('Save reading position')
  })

  it('refetches books and registers every write', () => {
    expect(bookWrites).toHaveLength(8)
    for (const write of bookWrites) expect(write.revalidate).toMatch(/^\/books/)
  })
})

describe('reading position write', () => {
  const save = {
    bookId: 'b',
    source: 'web',
    percent: 42,
    position: { href: 'ch2.xhtml', offset: 7 },
    readAt: '2026-10-02T12:00:00.000Z'
  }
  const stateAt = (readAt: string, updatedAt = '') =>
    create(GetReadingStateResponseSchema, {
      state: { source: 'kobo', percent: 10, readAt, updatedAt }
    })
  const apply = (data: unknown, key: unknown = swrKeys.readingState('b')) =>
    run(updateReadingProgressWrite, save, data, undefined, key)

  it('replaces an older or missing cached position', () => {
    for (const data of [
      stateAt('2026-10-01T00:00:00Z'),
      stateAt('', '2026-10-01T00:00:00Z'),
      create(GetReadingStateResponseSchema, {})
    ]) {
      const next = apply(data)
      expect(isMessage(next, GetReadingStateResponseSchema) && next.state).toMatchObject({
        source: 'web',
        percent: 42,
        position: { href: 'ch2.xhtml', offset: 7 },
        readAt: save.readAt,
        updatedAt: save.readAt
      })
    }
  })

  it('keeps a position read at the same time or later, like the server', () => {
    for (const data of [stateAt(save.readAt), stateAt('2026-10-03T00:00:00Z')]) {
      expect(apply(data)).toBe(data)
    }
  })

  it('leaves other books, keys and data alone', () => {
    const data = stateAt('')
    expect(apply(data, swrKeys.readingState('other'))).toBe(data)
    expect(apply(data, ['/books/other', 'b'])).toBe(data)
    expect(apply(data, swrKeys.books)).toBe(data)
    expect(apply('nope')).toBe('nope')
  })

  it('coalesces per book and skips the success refetch', () => {
    const bytes = updateReadingProgressWrite.encode(save)
    expect(updateReadingProgressWrite.coalesceKey(bytes)).toBe('b')
    expect(updateReadingProgressWrite.revalidate).toBe('/books/reading-state')
    expect(updateReadingProgressWrite.revalidateOnSuccess).toBe(false)
  })

  it('sends on the keepalive transport', async () => {
    await updateReadingProgressWrite.send(updateReadingProgressWrite.encode(save))
    expect(keepaliveTransport.unary).toHaveBeenCalledTimes(1)
    expect(transport.unary).not.toHaveBeenCalled()
  })
})
