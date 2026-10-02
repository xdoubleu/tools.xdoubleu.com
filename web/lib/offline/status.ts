// Connectivity state for the offline banner: whether the browser reports
// offline, and the oldest saved copy currently shown in place of live data.

export interface OfflineStatus {
  offline: boolean
  /** savedAt (ms) of the oldest stored entry served since the last live fetch. */
  servedFromCacheAt: number | null
}

const ONLINE: OfflineStatus = { offline: false, servedFromCacheAt: null }

let status: OfflineStatus = ONLINE
const listeners = new Set<() => void>()

function set(next: OfflineStatus) {
  if (next.offline === status.offline && next.servedFromCacheAt === status.servedFromCacheAt) return
  status = next
  listeners.forEach((l) => l())
}

function onConnectivityChange() {
  set({ ...status, offline: !navigator.onLine })
}

export function subscribe(listener: () => void): () => void {
  if (listeners.size === 0 && typeof window !== 'undefined') {
    window.addEventListener('online', onConnectivityChange)
    window.addEventListener('offline', onConnectivityChange)
    onConnectivityChange()
  }
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
    if (listeners.size === 0 && typeof window !== 'undefined') {
      window.removeEventListener('online', onConnectivityChange)
      window.removeEventListener('offline', onConnectivityChange)
    }
  }
}

export function getSnapshot(): OfflineStatus {
  return status
}

export function getServerSnapshot(): OfflineStatus {
  return ONLINE
}

export function markServedFromCache(savedAt: number) {
  const oldest =
    status.servedFromCacheAt === null ? savedAt : Math.min(status.servedFromCacheAt, savedAt)
  set({ ...status, servedFromCacheAt: oldest })
}

export function markLive() {
  set({ ...status, servedFromCacheAt: null })
}
