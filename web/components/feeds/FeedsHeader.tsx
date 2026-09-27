'use client'

import Link from 'next/link'
import { useEffect, useState } from 'react'
import { useFeeds } from '@/hooks/useFeeds'
import { useCurrentUser } from '@/hooks/useAuth'
import FeedManager from '@/components/feeds/FeedManager'
import { Button } from '@/components/ui/button'
import { PageHeader } from '@/components/ui/page-header'

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
      <PageHeader
        title="Feeds"
        actions={
          <>
            <Button asChild variant="link" className="min-w-11 sm:min-w-0">
              <Link href="/feeds/stats">Stats</Link>
            </Button>
            {isAdmin && (
              <Button asChild variant="link" className="min-w-11 sm:min-w-0">
                <Link href="/feeds/health">Health</Link>
              </Button>
            )}
            <Button asChild variant="link" className="min-w-11 sm:min-w-0">
              <Link href="/feeds/settings">Settings</Link>
            </Button>
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
          </>
        }
      />

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
