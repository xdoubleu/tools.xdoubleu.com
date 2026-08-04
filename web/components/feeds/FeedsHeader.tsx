'use client'

import Link from 'next/link'
import { useEffect, useState } from 'react'
import { useFeeds } from '@/hooks/useFeeds'
import FeedManager from '@/components/feeds/FeedManager'
import { Button } from '@/components/ui/button'

// The page header doubles as the entry point to feed management: the toggle is
// the page's only real action, so it sits next to the title rather than below
// the reader. Management is still secondary to reading unread items, so the
// panel starts collapsed — unless there are no feeds yet, in which case
// subscribing is the user's only next step. useFeeds() is a no-op extra
// request: FeedManager already calls it and SWR dedupes on the shared
// swrKeys.feeds key.
export default function FeedsHeader() {
  const { data } = useFeeds()
  const [open, setOpen] = useState(false)
  const [touched, setTouched] = useState(false)

  useEffect(() => {
    if (!touched && data && data.feeds.length === 0) setOpen(true)
  }, [touched, data])

  return (
    <>
      <div className="mb-6 flex items-center justify-between gap-4">
        <h1 className="text-3xl font-bold">Feeds</h1>
        <div className="flex items-center gap-2">
          <Link
            href="/feeds/stats"
            className="text-sm text-accent underline-offset-4 hover:underline"
          >
            Stats
          </Link>
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
      </div>

      {open && (
        <div id="manage-feeds-panel" className="mb-6 border-b border-border pb-6">
          <p className="mb-3 text-xs text-muted">
            Subscribe to blogs, news feeds, and email newsletters. New posts land here as items to
            read.
          </p>
          <FeedManager />
        </div>
      )}
    </>
  )
}
