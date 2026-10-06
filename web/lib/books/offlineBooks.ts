// Book files kept on the device for offline reading, keyed by book and
// format (never by the presigned URL, which expires). Sign-out wipes them with
// the rest of the offline store; the 30-day prune never touches them.
import { createServiceClient } from '@/lib/client'
import { BookFilesService } from '@/lib/gen/books/v1/files_pb'
import {
  deleteBookFile,
  listBookFileKeys,
  listBookFiles,
  loadBookFile,
  saveBookFile,
  type StoredBookInfo
} from '@/lib/offline/store'

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

function storeBookFile(ref: BookFileRef, blob: Blob): Promise<void> {
  return saveBookFile({ ...ref, blob, size: blob.size, savedAt: Date.now() })
}

/** Downloads the file through a fresh presigned URL and stores it. */
export async function downloadBookFile(ref: BookFileRef): Promise<Blob> {
  const blob = await fetchBookFile(ref)
  await storeBookFile(ref, blob)
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
      blob = await fetchBookFile(ref)
      void storeBookFile(ref, blob)
    } catch (err) {
      if (!stored) throw err
      blob = stored.blob
    }
  }
  // foliate-js sniffs some formats by file name.
  return new File([blob], `${ref.bookId}.${ref.format}`, { type: blob.type })
}

export function deleteStoredBook(bookId: string, format: string): Promise<void> {
  return deleteBookFile(bookId, format)
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
