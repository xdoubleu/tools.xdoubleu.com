'use client'

import { useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { useUpdateItem } from '@/hooks/useFeeds'

const UNDO_WINDOW_MS = 4000

interface FeedItemMarkReadButtonProps {
  itemId: string
  /** Called once the undo window has elapsed without the user reverting. */
  onSettled: (itemId: string) => void
}

// FeedItemMarkReadButton marks an item read with a brief Undo window
// (issue #476), now against the item's own persisted read_at (issue #734)
// instead of a library book's status.
export default function FeedItemMarkReadButton({ itemId, onSettled }: FeedItemMarkReadButtonProps) {
  const [justRead, setJustRead] = useState(false)
  const updateItem = useUpdateItem()
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    return () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current)
    }
  }, [])

  const handleMarkRead = async () => {
    setJustRead(true)
    try {
      await updateItem(itemId, { read: true })
      timeoutRef.current = setTimeout(() => onSettled(itemId), UNDO_WINDOW_MS)
    } catch {
      setJustRead(false)
    }
  }

  const handleUndo = async () => {
    if (timeoutRef.current) clearTimeout(timeoutRef.current)
    setJustRead(false)
    try {
      await updateItem(itemId, { read: false })
    } catch {
      // Best-effort: the row already reverted locally.
    }
  }

  if (justRead) {
    return (
      <span className="flex items-center gap-1 text-xs text-muted whitespace-nowrap">
        Marked as read
        <Button variant="link" size="sm" className="h-auto p-0 text-xs" onClick={handleUndo}>
          Undo
        </Button>
      </span>
    )
  }

  return (
    <Button variant="ghost" size="sm" onClick={handleMarkRead}>
      Mark read
    </Button>
  )
}
