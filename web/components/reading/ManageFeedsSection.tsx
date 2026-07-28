'use client'

import { useEffect, useState } from 'react'
import { useFeeds } from '@/hooks/useBookFeeds'
import FeedManager from '@/components/reading/FeedManager'
import { Button } from '@/components/ui/button'

// Feed management is secondary to reading unread items, so it starts
// collapsed — unless there are no feeds yet, in which case subscribing is the
// user's only next step. useFeeds() is a no-op extra request: FeedManager
// already calls it and SWR dedupes on the shared swrKeys.bookFeeds key.
export default function ManageFeedsSection() {
  const { data } = useFeeds()
  const [open, setOpen] = useState(false)
  const [touched, setTouched] = useState(false)

  useEffect(() => {
    if (!touched && data && data.feeds.length === 0) setOpen(true)
  }, [touched, data])

  return (
    <div className="mt-10 border-t border-border pt-8">
      <div className="mb-3 flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-muted">Manage feeds</h2>
        <Button
          variant="secondary"
          size="sm"
          aria-expanded={open}
          aria-controls="manage-feeds-panel"
          onClick={() => {
            setTouched(true)
            setOpen((v) => !v)
          }}
        >
          {open ? 'Hide' : 'Manage feeds'}
        </Button>
      </div>

      {open && (
        <div id="manage-feeds-panel">
          <p className="mb-3 text-xs text-muted">
            Subscribe to blogs, news feeds, and email newsletters. New posts are converted to EPUB
            and added to your reading library.
          </p>
          <FeedManager />
        </div>
      )}
    </div>
  )
}
