'use client'

import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { useUpdateItem } from '@/hooks/useFeeds'

const UNDO_WINDOW_MS = 4000

interface FeedItemMarkReadButtonProps {
  itemId: string
  /** Called synchronously on click, before the read state is persisted. */
  onMarkRead: (itemId: string) => void
  /** Called once the undo window has elapsed without the user reverting. */
  onSettled: (itemId: string) => void
}

export interface FeedItemMarkReadHandle {
  /** Marks the item read as if the button were clicked; no-ops once already triggered. */
  markRead: () => void
}

// FeedItemMarkReadButton marks an item read with a brief Undo window
// (issue #476), now against the item's own persisted read_at (issue #734)
// instead of a library book's status. Also triggerable imperatively (issue
// #716) so scrolling to the end of an item auto-marks it read via the same
// undo flow as the manual button.
const FeedItemMarkReadButton = forwardRef<FeedItemMarkReadHandle, FeedItemMarkReadButtonProps>(
  function FeedItemMarkReadButton({ itemId, onMarkRead, onSettled }, ref) {
    const [justRead, setJustRead] = useState(false)
    const updateItem = useUpdateItem()
    const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
    const triggeredRef = useRef(false)

    useEffect(() => {
      return () => {
        if (timeoutRef.current) clearTimeout(timeoutRef.current)
      }
    }, [])

    const handleMarkRead = async () => {
      if (triggeredRef.current) return
      triggeredRef.current = true
      setJustRead(true)
      onMarkRead(itemId)
      try {
        await updateItem(itemId, { read: true })
        timeoutRef.current = setTimeout(() => onSettled(itemId), UNDO_WINDOW_MS)
      } catch {
        triggeredRef.current = false
        setJustRead(false)
      }
    }

    useImperativeHandle(ref, () => ({ markRead: handleMarkRead }))

    const handleUndo = async () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current)
      triggeredRef.current = false
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
)

export default FeedItemMarkReadButton
