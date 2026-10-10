// Book files kept on the device for offline reading, keyed by book and
// format (never by the presigned URL, which expires). Sign-out wipes them with
// the rest of the offline store; the 30-day prune never touches them.
import { createServiceClient } from '@/lib/client'
import { BookFilesService } from '@/lib/gen/books/v1/files_pb'
import {
  clearBookFiles,
  deleteBookFile,
  listBookFileKeys,
  listBookFiles,
  loadBookFile,
  saveBookFile,
  storeGeneration,
  type StoredBookInfo
} from '@/lib/offline/store'
import { evictAllWebPubs, evictWebPub } from './webpubCache'

export { subscribeBookFiles as subscribeStoredBooks } from '@/lib/offline/store'

export interface BookFileRef {
  bookId: string
  format: string
  /** `UserBook.fileVersions[format]`; '' when unknown. */
  version: string
}

async function fetchBookFile({ bookId, format }: BookFileRef): Promise<Blob> {
  const { url } = await createServiceClient(BookFilesService).getBookFile({ bookId, format })
  const res = await fetch(url)
  if (!res.ok) throw new Error(`book download failed: ${res.status}`)
  return res.blob()
}

// A file fetched for a session that was wiped meanwhile is never stored.
async function storeBookFile(ref: BookFileRef, blob: Blob, generation: number): Promise<void> {
  if (generation !== storeGeneration()) return
  await saveBookFile({ ...ref, blob, size: blob.size, savedAt: Date.now() })
}

/** Downloads the file through a fresh presigned URL and stores it. */
export async function downloadBookFile(ref: BookFileRef): Promise<Blob> {
  const generation = storeGeneration()
  const blob = await fetchBookFile(ref)
  await storeBookFile(ref, blob, generation)
  return blob
}

/**
 * The file to open: the stored copy while its version matches, else a fresh
 * download, stored in the background. A stale copy is still opened when the
 * download fails (offline).
 */
export async function openBookFile(ref: BookFileRef): Promise<File> {
  const stored = await loadBookFile(ref.bookId, ref.format)
  let blob = stored?.version === ref.version ? stored.blob : undefined
  if (!blob) {
    try {
      const generation = storeGeneration()
      blob = await fetchBookFile(ref)
      void storeBookFile(ref, blob, generation)
    } catch (err) {
      if (!stored) throw err
      blob = stored.blob
    }
  }
  // foliate-js sniffs some formats by file name.
  return new File([blob], `${ref.bookId}.${ref.format}`, { type: blob.type })
}

export async function deleteStoredBook(bookId: string, format: string): Promise<void> {
  await deleteBookFile(bookId, format)
  if (format === 'epub') await evictWebPub(bookId).catch(() => undefined)
}

export async function deleteAllStoredBooks(): Promise<void> {
  await clearBookFiles()
  await evictAllWebPubs().catch(() => undefined)
}

export function listStoredBooks(): Promise<StoredBookInfo[]> {
  return listBookFiles()
}

/** The stored file's version, or null when the format isn't stored. */
export async function storedBookVersion(bookId: string, format: string): Promise<string | null> {
  return (await loadBookFile(bookId, format))?.version ?? null
}

/** IDs of books with at least one stored format. */
export async function storedBookIds(): Promise<Set<string>> {
  return new Set((await listBookFileKeys()).map((k) => k.bookId))
}

/** Total bytes of stored book files. */
export async function storedBooksSize(): Promise<number> {
  return (await listBookFiles()).reduce((sum, f) => sum + f.size, 0)
}
