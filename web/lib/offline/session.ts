import { logger } from '@/lib/logger'
import { resetOutbox } from './outbox'
import { PAGE_CACHE } from './serviceWorker'
import { clearStore, getOwner, pruneEntries, setOwner } from './store'

const MAX_ENTRY_AGE_MS = 30 * 24 * 60 * 60 * 1000

export function registerServiceWorker() {
  if (process.env.NODE_ENV !== 'production' || !('serviceWorker' in navigator)) return
  navigator.serviceWorker.register('/sw.js').catch((err: unknown) => {
    logger.warn('service worker registration failed', { error: String(err) })
  })
}

/** Asks the service worker to save `url` for offline use. */
export function savePageForOffline(url: string) {
  if (!('serviceWorker' in navigator)) return
  void navigator.serviceWorker.ready.then((reg) =>
    reg.active?.postMessage({ type: 'save-page', url })
  )
}

/** Deletes every saved page, response and queued write; run on sign-out. */
export async function clearOfflineData() {
  if ('serviceWorker' in navigator)
    navigator.serviceWorker.controller?.postMessage({ type: 'clear' })
  await Promise.all([
    clearStore(),
    typeof caches === 'undefined' ? undefined : caches.delete(PAGE_CACHE).catch(() => false)
  ])
  resetOutbox()
}

/** Wipes data saved for a different user, then drops entries older than 30 days. */
export async function claimOfflineData(userId: string) {
  const owner = await getOwner()
  if (owner !== undefined && owner !== userId) await clearOfflineData()
  if (owner !== userId) await setOwner(userId)
  await pruneEntries(Date.now() - MAX_ENTRY_AGE_MS)
}
