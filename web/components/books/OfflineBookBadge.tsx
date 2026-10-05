'use client'

import { Badge } from '@/components/ui/badge'
import { useStoredBookIds } from '@/hooks/useOfflineBooks'

/** Marks a book whose file is stored on this device for offline reading. */
export default function OfflineBookBadge({ bookId }: { bookId: string }) {
  if (!useStoredBookIds().has(bookId)) return null
  return <Badge variant="success">Available offline</Badge>
}
