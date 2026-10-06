'use client'

import { useEffect } from 'react'
import { useLibrary } from '@/hooks/useBooks'
import { syncOfflineBooks } from '@/lib/books/offlineSync'
import { servedFromCache } from '@/lib/offline/persist'
import { swrKeys } from '@/lib/swrKeys'

/**
 * Downloads currently-reading books and evicts unneeded ones after each
 * library fetch. Only live data counts: never server-rendered fallback data
 * before its revalidation, a failed fetch, or a saved copy served offline.
 */
export default function OfflineBooksSync() {
  // Mounted app-wide: focus refetches are left to the books pages.
  const { data, error, isValidating } = useLibrary({ revalidateOnFocus: false })
  useEffect(() => {
    const library = data?.library
    if (!library || error || isValidating) return
    if (servedFromCache(swrKeys.books)) return
    void syncOfflineBooks(library)
  }, [data, error, isValidating])
  return null
}
