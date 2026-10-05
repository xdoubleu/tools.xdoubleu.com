// Book files kept on the device for offline reading, keyed by book and
// format (never by the presigned URL, which expires). Sign-out wipes them with
// the rest of the offline store; the 30-day prune never touches them.
import { createServiceClient } from '@/lib/client'
import { BookFilesService } from '@/lib/gen/books/v1/files_pb'
import {
  deleteBookFile,
  listBookFiles,
  loadBookFile,
  saveBookFile,
  type StoredBookInfo
} from '@/lib/offline/store'

export interface BookFileRef {
  bookId: string
  format: string
  /** `UserBook.fileVersions[format]`; '' when unknown. */
  version: string
}

const listeners = new Set<() => void>()
const notify = () => listeners.forEach((l) => l())

/** Runs `listener` whenever a book is stored or deleted. */
export function subscribeStoredBooks(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

/** Downloads the file through a fresh presigned URL and stores it. */
export async function downloadBookFile({ bookId, format, version }: BookFileRef): Promise<Blob> {
  const { url } = await createServiceClient(BookFilesService).getBookFile({ bookId, format })
  const res = await fetch(url)
  if (!res.ok) throw new Error(`book download failed: ${res.status}`)
  const blob = await res.blob()
  await saveBookFile({ bookId, format, version, blob, size: blob.size, savedAt: Date.now() })
  notify()
  return blob
}

/**
 * The file to open: the stored copy while its version matches, else a fresh
 * download. A stale copy is still opened when the download fails (offline).
 */
export async function openBookFile(ref: BookFileRef): Promise<File> {
  const stored = await loadBookFile(ref.bookId, ref.format)
  let blob = stored?.version === ref.version ? stored.blob : undefined
  if (!blob) {
    try {
      blob = await downloadBookFile(ref)
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
  notify()
}

export function listStoredBooks(): Promise<StoredBookInfo[]> {
  return listBookFiles()
}

/** Total bytes of stored book files. */
export async function storedBooksSize(): Promise<number> {
  return (await listBookFiles()).reduce((sum, f) => sum + f.size, 0)
}
