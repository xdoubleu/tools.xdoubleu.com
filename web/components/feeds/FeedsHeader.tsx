'use client'

import Link from 'next/link'
import { useEffect, useState } from 'react'
import { useFeeds } from '@/hooks/useFeeds'
import { useCurrentUser } from '@/hooks/useAuth'
import FeedManager from '@/components/feeds/FeedManager'
import { Button } from '@/components/ui/button'

// The feed-management toggle sits by the title; the panel starts collapsed
// unless there are no feeds. useFeeds() is deduped by SWR.
export default function FeedsHeader() {
  const { data } = useFeeds()
  const { data: currentUser } = useCurrentUser()
  const isAdmin = currentUser?.role === 'admin'
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
          {isAdmin && (
            <Link
              href="/feeds/health"
              className="text-sm text-accent underline-offset-4 hover:underline"
            >
              Health
            </Link>
          )}
          <Link
            href="/feeds/settings"
            className="text-sm text-accent underline-offset-4 hover:underline"
          >
            Settings
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
