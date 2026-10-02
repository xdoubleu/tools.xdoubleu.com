import { create, isMessage } from '@bufbuild/protobuf'
import { defineOfflineWrite } from '@/lib/offline/registry'
import { swrKeys } from '@/lib/swrKeys'
import {
  BookShelfSchema,
  GetLibraryResponseSchema,
  LibraryService,
  type LibraryResponse,
  type UserBook
} from '@/lib/gen/books/v1/library_pb'
import { CatalogService } from '@/lib/gen/books/v1/catalog_pb'

const library = LibraryService.method
const catalog = CatalogService.method
const REVALIDATE = '/books'
const FAVOURITE = 'favourite'
const READ = 'read'

/** The built-in list a status files under; other statuses are shelves. */
function groupOf(status: string): 'reading' | 'wishlist' | 'finished' | undefined {
  switch (status) {
    case 'currently-reading':
      return 'reading'
    case 'to-read':
      return 'wishlist'
    case READ:
      return 'finished'
    default:
      return undefined
  }
}

const title = (ub: UserBook) => ub.book?.title ?? ''

/** Inserts `ub` before the first book with a later title (the API sorts by title). */
function insertSorted(list: UserBook[], ub: UserBook): UserBook[] {
  const at = list.findIndex((b) => title(b).localeCompare(title(ub)) > 0)
  return at < 0 ? [...list, ub] : [...list.slice(0, at), ub, ...list.slice(at)]
}

function allBooks(lib: LibraryResponse): UserBook[] {
  return [...lib.reading, ...lib.wishlist, ...lib.finished, ...lib.shelves.flatMap((s) => s.books)]
}

function without(lib: LibraryResponse, bookId: string): LibraryResponse {
  const keep = (list: UserBook[]) => list.filter((b) => b.bookId !== bookId)
  return {
    ...lib,
    reading: keep(lib.reading),
    wishlist: keep(lib.wishlist),
    finished: keep(lib.finished),
    shelves: lib.shelves.map((s) => ({ ...s, books: keep(s.books) }))
  }
}

/** Files `ub` under its status: a built-in list, else a shelf named after it. */
function place(lib: LibraryResponse, ub: UserBook): LibraryResponse {
  const next = without(lib, ub.bookId)
  const group = groupOf(ub.status)
  if (group) return { ...next, [group]: insertSorted(next[group], ub) }
  const shelves = next.shelves.some((s) => s.name === ub.status)
    ? next.shelves
    : [...next.shelves, create(BookShelfSchema, { name: ub.status })].sort((a, b) =>
        a.name < b.name ? -1 : 1
      )
  return {
    ...next,
    shelves: shelves.map((s) =>
      s.name === ub.status ? { ...s, books: insertSorted(s.books, ub) } : s
    )
  }
}

/** Applies `fn` to the cached library's copy of `bookId`, refiling it if moved. */
function onBook(
  key: unknown,
  data: unknown,
  bookId: string,
  fn: (ub: UserBook) => UserBook
): unknown {
  if (key !== swrKeys.books || !isMessage(data, GetLibraryResponseSchema) || !data.library) {
    return data
  }
  const lib = data.library
  const current = allBooks(lib).find((b) => b.bookId === bookId)
  if (!current) return data
  const next = fn(current)
  if (next.status !== current.status || title(next) !== title(current)) {
    return { ...data, library: place(lib, next) }
  }
  const swap = (list: UserBook[]) => list.map((b) => (b.bookId === bookId ? next : b))
  return {
    ...data,
    library: {
      ...lib,
      reading: swap(lib.reading),
      wishlist: swap(lib.wishlist),
      finished: swap(lib.finished),
      shelves: lib.shelves.map((s) => ({ ...s, books: swap(s.books) }))
    }
  }
}

function withTag(tags: string[], tag: string, enabled: boolean): string[] {
  const rest = tags.filter((t) => t !== tag)
  return enabled ? [...rest, tag] : rest
}

/** Mirrors the API's parseRating; an unset rating keeps the stored one. */
function parseRating(raw: string, current: number): number {
  const n = Number(raw)
  return Number.isInteger(n) && n >= 1 && n <= 5 ? n : current
}

const normalizeISBN = (raw: string) => raw.replace(/[- ]/g, '')

export const updateBookStatusWrite = defineOfflineWrite({
  method: library.updateBookStatus,
  apply: (key, data, req, hint) =>
    onBook(key, data, req.bookId, (ub) => {
      // A move to read logs one finish, stamped when it was queued.
      let finishedAt: string[] = []
      if (req.status === READ) {
        finishedAt =
          ub.status === READ
            ? ub.finishedAt
            : [...ub.finishedAt, typeof hint === 'string' ? hint : '']
      }
      return {
        ...ub,
        status: req.status,
        tags: withTag(ub.tags, FAVOURITE, req.favourite),
        rating: parseRating(req.rating, ub.rating),
        finishedAt
      }
    }),
  describe: () => 'Update a book’s status',
  revalidate: REVALIDATE
})

export const updateFinishedAtWrite = defineOfflineWrite({
  method: library.updateFinishedAt,
  apply: (key, data, req) =>
    onBook(key, data, req.bookId, (ub) => ({
      ...ub,
      finishedAt: req.finishedAt.filter(Boolean).map((d) => `${d}T00:00:00Z`)
    })),
  describe: () => 'Edit a book’s read dates',
  revalidate: REVALIDATE
})

export const updateProgressWrite = defineOfflineWrite({
  method: library.updateProgress,
  apply: (key, data, req) =>
    req.progressMode !== 'pages' && req.progressMode !== 'percent'
      ? data
      : onBook(key, data, req.bookId, (ub) => ({
          ...ub,
          progressMode: req.progressMode,
          currentPage: Math.max(req.currentPage, 0),
          progressPercent: Math.min(Math.max(req.progressPercent, 0), 100)
        })),
  describe: () => 'Update reading progress',
  revalidate: REVALIDATE
})

export const setBookTagWrite = defineOfflineWrite({
  method: library.setBookTag,
  apply: (key, data, req) =>
    onBook(key, data, req.bookId, (ub) => ({
      ...ub,
      tags: withTag(ub.tags, req.tag, req.enabled)
    })),
  describe: (req) => `${req.enabled ? 'Add' : 'Remove'} the “${req.tag}” tag`,
  revalidate: REVALIDATE
})

export const removeBookWrite = defineOfflineWrite({
  method: library.removeBook,
  apply: (key, data, req) =>
    key === swrKeys.books && isMessage(data, GetLibraryResponseSchema) && data.library
      ? { ...data, library: without(data.library, req.bookId) }
      : data,
  describe: () => 'Remove a book',
  revalidate: REVALIDATE
})

export const setBookISBNWrite = defineOfflineWrite({
  method: catalog.setBookISBN,
  apply: (key, data, req) =>
    onBook(key, data, req.bookId, (ub) =>
      ub.book ? { ...ub, book: { ...ub.book, isbn13: normalizeISBN(req.isbn13) } } : ub
    ),
  describe: () => 'Set a book’s ISBN',
  revalidate: REVALIDATE
})

export const updateBookWrite = defineOfflineWrite({
  method: catalog.updateBook,
  apply: (key, data, req) =>
    onBook(key, data, req.bookId, (ub) => {
      const meta = req.metadata
      if (!ub.book || !meta) return ub
      return {
        ...ub,
        book: {
          ...ub.book,
          title: meta.title,
          authors: meta.authors,
          isbn13: normalizeISBN(meta.isbn13),
          description: meta.description,
          pageCount: meta.pageCount,
          // A new cover is fetched by the API; until then the old one shows.
          coverUrl: meta.coverUrl ? ub.book.coverUrl : ''
        }
      }
    }),
  describe: (req) => `Edit “${req.metadata?.title ?? 'a book'}”`,
  revalidate: REVALIDATE
})

export const bookWrites = [
  updateBookStatusWrite,
  updateFinishedAtWrite,
  updateProgressWrite,
  setBookTagWrite,
  removeBookWrite,
  setBookISBNWrite,
  updateBookWrite
]
