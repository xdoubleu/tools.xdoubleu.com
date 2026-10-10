// Keeps currently-reading books on the device in their original format, and
// removes stored files the library no longer needs.
import { flattenLibrary } from './bookShelves'
import { warmReaderModules } from './readerModules'
import {
  deleteStoredBook,
  downloadBookFile,
  listStoredBooks,
  type BookFileRef
} from './offlineBooks'
import { pickReaderFormat } from './readerSettings'
import { isWebPubWarm, warmWebPub } from './webpubCache'
import type { LibraryResponse, UserBook } from '@/lib/gen/books/v1/library_pb'
import { savePageForOffline } from '@/lib/offline/session'
import type { StoredBookInfo } from '@/lib/offline/store'

const FINISHED = 'read'
const DOWNLOAD_GAP_MS = 3000
const FAILURE_BACKOFF_MS = 30 * 60 * 1000

interface PlannedDownload extends BookFileRef {
  /** The book's reader page, saved for offline use once the file is stored. */
  readerPath: string
}

export interface OfflinePlan {
  downloads: PlannedDownload[]
  evictions: { bookId: string; format: string }[]
}

/** The file the reader opens for the book: its original EPUB (preferred) or PDF. */
export function preferredBookFormat(userBook: UserBook): string | null {
  return pickReaderFormat(userBook.formats, null)
}

/**
 * Downloads: currently-reading books whose original file is ready but not
 * stored at its current version. Evictions: every file of a finished book or
 * one no longer in the library, and any stored KEPUB (the reader no longer
 * opens one).
 */
export function planOfflineBooks(library: LibraryResponse, stored: StoredBookInfo[]): OfflinePlan {
  const books = new Map(flattenLibrary(library).map((ub) => [ub.bookId, ub]))
  const storedVersions = new Map<string, Map<string, string>>()
  for (const { bookId, format, version } of stored) {
    const formats = storedVersions.get(bookId) ?? new Map<string, string>()
    formats.set(format, version)
    storedVersions.set(bookId, formats)
  }

  const evictions: OfflinePlan['evictions'] = []
  for (const [bookId, formats] of storedVersions) {
    const userBook = books.get(bookId)
    const evictAll = !userBook || userBook.status === FINISHED
    for (const format of formats.keys()) {
      if (evictAll || format === 'kepub') evictions.push({ bookId, format })
    }
  }

  const downloads: PlannedDownload[] = []
  for (const userBook of library.reading) {
    const storedFormats = storedVersions.get(userBook.bookId)
    const format = preferredBookFormat(userBook)
    const version = format ? userBook.fileVersions[format] : undefined
    if (!format || !version || storedFormats?.get(format) === version) continue
    downloads.push({
      bookId: userBook.bookId,
      format,
      version,
      readerPath: `/books/${userBook.id}/read`
    })
  }
  return { downloads, evictions }
}

interface NetworkInformation {
  saveData?: boolean
}

/** Online, without the browser's data saver. */
export function canPrefetch(): boolean {
  const connection = (navigator as Navigator & { connection?: NetworkInformation }).connection
  return navigator.onLine && connection?.saveData !== true
}

// A download that failed (e.g. a missing file) waits before it is retried.
const failedAt = new Map<string, number>()
const refKey = (ref: BookFileRef) => `${ref.bookId}:${ref.format}:${ref.version}`

async function evict(library: LibraryResponse): Promise<StoredBookInfo[]> {
  const stored = await listStoredBooks()
  const { evictions } = planOfflineBooks(library, stored)
  for (const { bookId, format } of evictions) await deleteStoredBook(bookId, format)
  return stored.filter(
    (s) => !evictions.some((e) => e.bookId === s.bookId && e.format === s.format)
  )
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

// An EPUB stored before the offline copy existed gets one on the next sync.
async function warmStoredEPUBs(stored: StoredBookInfo[]) {
  for (const { bookId, format } of stored) {
    if (format !== 'epub' || !canPrefetch()) continue
    if (await isWebPubWarm(bookId).catch(() => true)) continue
    await warmWebPub(bookId).catch(() => undefined)
  }
}

async function syncOnce(library: LibraryResponse, gapMs: number) {
  const stored = await evict(library)
  const { downloads } = planOfflineBooks(library, stored)
  let downloaded = 0
  for (const { readerPath, ...ref } of downloads) {
    if (Date.now() - (failedAt.get(refKey(ref)) ?? -Infinity) < FAILURE_BACKOFF_MS) continue
    if (downloaded > 0) await sleep(gapMs)
    if (!canPrefetch()) break
    try {
      await downloadBookFile(ref)
      downloaded++
      if (ref.format === 'epub') await warmWebPub(ref.bookId).catch(() => undefined)
      savePageForOffline(readerPath)
    } catch {
      // Losing the connection isn't the file's fault.
      if (!canPrefetch()) break
      failedAt.set(refKey(ref), Date.now())
    }
  }
  // Again, so a newly stored format replaces the old one.
  const remaining = await evict(library)
  await warmStoredEPUBs(remaining)
  if (remaining.length > 0 && canPrefetch()) await warmReaderModules()
}

let running: Promise<void> | null = null
let queued: LibraryResponse | null = null

/**
 * Applies `planOfflineBooks` to `library`, which must be live: evicting
 * against stale or partial data would delete books still in use. Downloads
 * run one at a time, `gapMs` apart, and only while `canPrefetch`. A call
 * during a run queues one rerun with the newest library.
 */
export function syncOfflineBooks(
  library: LibraryResponse,
  { gapMs = DOWNLOAD_GAP_MS }: { gapMs?: number } = {}
): Promise<void> {
  if (running) {
    queued = library
    return running
  }
  running = (async () => {
    // Cleared in the same tick as the last `queued` check, so no call is lost.
    try {
      let next: LibraryResponse | null = library
      while (next) {
        queued = null
        await syncOnce(next, gapMs)
        next = queued
      }
    } finally {
      running = null
    }
  })()
  return running
}
