/**
 * Maximum raw file size accepted by the server (250 MB).
 * Keep in sync with MaxUploadBytes in api/apps/backlog/internal/services/book_files.go.
 */
export const MAX_UPLOAD_BYTES = 250 * 1024 * 1024

/** Accepted book file extensions. */
const BOOK_EXTENSIONS = ['.epub', '.pdf']

/** Returns true if the File has an accepted extension. */
export function isBookFile(file: File): boolean {
  const lower = file.name.toLowerCase()
  return BOOK_EXTENSIONS.some((ext) => lower.endsWith(ext))
}

// --- FileSystem Entry helpers ---

function readDirEntries(reader: FileSystemDirectoryReader): Promise<FileSystemEntry[]> {
  return new Promise((resolve, reject) => {
    const all: FileSystemEntry[] = []
    function next() {
      reader.readEntries((batch) => {
        if (batch.length === 0) resolve(all)
        else {
          all.push(...batch)
          next()
        }
      }, reject)
    }
    next()
  })
}

function isFileEntry(e: FileSystemEntry): e is FileSystemFileEntry {
  return e.isFile
}

function isDirEntry(e: FileSystemEntry): e is FileSystemDirectoryEntry {
  return e.isDirectory
}

async function collectFiles(entry: FileSystemEntry): Promise<File[]> {
  if (isFileEntry(entry)) {
    return new Promise<File[]>((resolve, reject) => entry.file((f) => resolve([f]), reject))
  }
  if (isDirEntry(entry)) {
    const reader = entry.createReader()
    const children = await readDirEntries(reader)
    const nested = await Promise.all(children.map(collectFiles))
    return nested.flat()
  }
  return []
}

/**
 * Extract File objects from a DataTransfer, traversing dropped folders
 * recursively via the FileSystem Entry API when available.
 */
export async function filesFromDataTransfer(dt: DataTransfer): Promise<File[]> {
  if (dt.items && dt.items.length > 0 && typeof dt.items[0].webkitGetAsEntry === 'function') {
    const entries = Array.from(dt.items)
      .map((item) => item.webkitGetAsEntry())
      .filter((e): e is FileSystemEntry => e !== null)
    const nested = await Promise.all(entries.map(collectFiles))
    return nested.flat()
  }
  return Array.from(dt.files)
}
