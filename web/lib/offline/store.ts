// IndexedDB storage for offline use. Every call resolves (undefined or a
// no-op) where IndexedDB is missing or fails, so callers never need a guard.

const DB_NAME = 'tools-offline'
const DB_VERSION = 1
const ENTRIES = 'swr'
const META = 'meta'
const OWNER_KEY = 'owner'

export interface StoredEntry {
  data: unknown
  savedAt: number
}

let dbPromise: Promise<IDBDatabase> | null = null

function openDb(): Promise<IDBDatabase> {
  dbPromise ??= new Promise<IDBDatabase>((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => {
      req.result.createObjectStore(ENTRIES)
      req.result.createObjectStore(META)
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
  await withStore(ENTRIES, 'readwrite', (s) => s.clear())
  await withStore(META, 'readwrite', (s) => s.clear())
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
