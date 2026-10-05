// IndexedDB storage for offline use. Every call resolves (undefined or a
// no-op) where IndexedDB is missing or fails, so callers never need a guard.

const DB_NAME = 'tools-offline'
const DB_VERSION = 3
const ENTRIES = 'swr'
const META = 'meta'
const OUTBOX = 'outbox'
const FAILED = 'failed'
const BOOKS = 'books'
const OWNER_KEY = 'owner'

export interface StoredEntry {
  data: unknown
  savedAt: number
}

/** A write waiting to be sent, keyed by `seq` (insertion order). */
export interface QueuedWrite {
  seq?: number
  /** `<service typeName>/<method localName>`. */
  writeId: string
  request: Uint8Array
  /** Write-specific data for its optimistic update, e.g. display names. */
  hint?: unknown
  createdAt: number
}

/** A write the server rejected, kept until the user dismisses it. */
export interface FailedWrite {
  seq?: number
  description: string
  reason: string
}

/** A downloaded book file, keyed by book and format. */
export interface StoredBookFile {
  bookId: string
  format: string
  /** The server's file version (`UserBook.fileVersions`); '' when unknown. */
  version: string
  blob: Blob
  size: number
  savedAt: number
}

export type StoredBookInfo = Omit<StoredBookFile, 'blob'>

let dbPromise: Promise<IDBDatabase> | null = null

function openDb(): Promise<IDBDatabase> {
  dbPromise ??= new Promise<IDBDatabase>((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => {
      const db = req.result
      for (const name of [ENTRIES, META, BOOKS]) {
        if (!db.objectStoreNames.contains(name)) db.createObjectStore(name)
      }
      for (const name of [OUTBOX, FAILED]) {
        if (!db.objectStoreNames.contains(name)) {
          db.createObjectStore(name, { keyPath: 'seq', autoIncrement: true })
        }
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(new Error('indexedDB open failed', { cause: req.error }))
  }).catch((err: unknown) => {
    // Retry the open on the next call.
    dbPromise = null
    throw err
  })
  return dbPromise
}

async function withStore<T>(
  storeName: string,
  mode: IDBTransactionMode,
  fn: (store: IDBObjectStore) => IDBRequest<T>
): Promise<T | undefined> {
  if (typeof indexedDB === 'undefined') return undefined
  try {
    const db = await openDb()
    return await new Promise<T | undefined>((resolve, reject) => {
      const tx = db.transaction(storeName, mode)
      const req = fn(tx.objectStore(storeName))
      tx.oncomplete = () => resolve(req.result)
      // Errors abort the transaction.
      tx.onabort = () => reject(new Error('indexedDB transaction aborted', { cause: tx.error }))
    })
  } catch {
    return undefined
  }
}

export async function loadEntry(key: string): Promise<StoredEntry | undefined> {
  const entry = await withStore<unknown>(ENTRIES, 'readonly', (s) => s.get(key))
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- only saveEntry writes this store
  return entry as StoredEntry | undefined
}

export async function saveEntry(key: string, data: unknown): Promise<void> {
  const entry: StoredEntry = { data, savedAt: Date.now() }
  await withStore(ENTRIES, 'readwrite', (s) => s.put(entry, key))
}

export async function getOwner(): Promise<string | undefined> {
  const owner = await withStore<unknown>(META, 'readonly', (s) => s.get(OWNER_KEY))
  return typeof owner === 'string' ? owner : undefined
}

export async function setOwner(userId: string): Promise<void> {
  await withStore(META, 'readwrite', (s) => s.put(userId, OWNER_KEY))
}

export async function clearStore(): Promise<void> {
  for (const name of [ENTRIES, META, OUTBOX, FAILED, BOOKS]) {
    await withStore(name, 'readwrite', (s) => s.clear())
  }
}

/** Appends a write and resolves to its `seq`. */
export async function addQueued(write: QueuedWrite): Promise<number | undefined> {
  const seq = await withStore<IDBValidKey>(OUTBOX, 'readwrite', (s) => s.add(write))
  return typeof seq === 'number' ? seq : undefined
}

export async function listQueued(): Promise<QueuedWrite[]> {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- only addQueued writes this store
  return ((await withStore(OUTBOX, 'readonly', (s) => s.getAll())) ?? []) as QueuedWrite[]
}

export async function deleteQueued(seq: number): Promise<void> {
  await withStore(OUTBOX, 'readwrite', (s) => s.delete(seq))
}

export async function addFailed(write: FailedWrite): Promise<void> {
  await withStore(FAILED, 'readwrite', (s) => s.add(write))
}

export async function listFailed(): Promise<FailedWrite[]> {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- only addFailed writes this store
  return ((await withStore(FAILED, 'readonly', (s) => s.getAll())) ?? []) as FailedWrite[]
}

export async function clearFailed(): Promise<void> {
  await withStore(FAILED, 'readwrite', (s) => s.clear())
}

/** Deletes entries saved before `cutoff` (ms since epoch). */
export async function pruneEntries(cutoff: number): Promise<void> {
  await withStore(ENTRIES, 'readwrite', (s) => {
    const cursorReq = s.openCursor()
    cursorReq.onsuccess = () => {
      const cursor = cursorReq.result
      if (!cursor) return
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- only saveEntry writes this store
      if ((cursor.value as StoredEntry).savedAt < cutoff) cursor.delete()
      cursor.continue()
    }
    return cursorReq
  })
}

const bookKey = (bookId: string, format: string) => `${bookId}:${format}`

export async function saveBookFile(file: StoredBookFile): Promise<void> {
  await withStore(BOOKS, 'readwrite', (s) => s.put(file, bookKey(file.bookId, file.format)))
}

export async function loadBookFile(
  bookId: string,
  format: string
): Promise<StoredBookFile | undefined> {
  const file = await withStore<unknown>(BOOKS, 'readonly', (s) => s.get(bookKey(bookId, format)))
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- only saveBookFile writes this store
  return file as StoredBookFile | undefined
}

export async function deleteBookFile(bookId: string, format: string): Promise<void> {
  await withStore(BOOKS, 'readwrite', (s) => s.delete(bookKey(bookId, format)))
}

/** Every stored book file, without its bytes. */
export async function listBookFiles(): Promise<StoredBookInfo[]> {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- only saveBookFile writes this store
  const files = ((await withStore(BOOKS, 'readonly', (s) => s.getAll())) ?? []) as StoredBookFile[]
  return files.map(({ bookId, format, version, size, savedAt }) => ({
    bookId,
    format,
    version,
    size,
    savedAt
  }))
}
