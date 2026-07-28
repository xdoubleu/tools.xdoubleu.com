'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import AddFeedForm from '@/components/reading/AddFeedForm'
import { useFeeds, useDeleteFeed, useRefreshFeed } from '@/hooks/useBookFeeds'
import type { Feed } from '@/lib/gen/reading/v1/feeds_pb'

function FeedRow({ feed }: { feed: Feed }) {
  const deleteFeed = useDeleteFeed()
  const refreshFeed = useRefreshFeed()
  const [busy, setBusy] = useState(false)
  const [status, setStatus] = useState('')

  const run = async (action: () => Promise<string | void>) => {
    setBusy(true)
    setStatus('')
    try {
      const message = await action()
      if (message) setStatus(message)
    } catch {
      setStatus('Action failed.')
    } finally {
      setBusy(false)
    }
  }

  const isEmailFeed = feed.sourceType === 'email'

  return (
    <li className="rounded-2xl border border-border bg-card p-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium">
            {feed.title || feed.url || 'Email newsletter'}
          </p>
          {!isEmailFeed && <p className="truncate text-xs text-muted">{feed.url}</p>}
        </div>
        {!isEmailFeed && (
          <Button
            size="sm"
            variant="secondary"
            disabled={busy}
            onClick={() =>
              void run(async () => {
                const resp = await refreshFeed(feed.id)
                return `Ingested ${resp.ingested} item(s).`
              })
            }
          >
            Refresh
          </Button>
        )}
        <Button
          size="sm"
          variant="destructive"
          disabled={busy}
          onClick={() => void run(() => deleteFeed(feed.id))}
        >
          Remove
        </Button>
      </div>
      <div className="mt-1 space-y-0.5">
        {feed.lastError && (
          <p className="text-xs text-danger">
            {isEmailFeed ? 'Last email failed' : 'Last poll failed'}: {feed.lastError}
          </p>
        )}
        {feed.lastFetchedAt && !feed.lastError && (
          <p className="text-xs text-muted">
            {isEmailFeed ? 'Verified — last email received' : 'Last fetched'}{' '}
            {new Date(feed.lastFetchedAt).toLocaleString()}
          </p>
        )}
        {isEmailFeed && !feed.lastFetchedAt && !feed.lastError && (
          <p className="text-xs text-muted">
            Waiting for your first email — give the address above to the newsletter, then check back
            here.
          </p>
        )}
        {status && <p className="text-xs text-muted">{status}</p>}
      </div>
    </li>
  )
}

// FeedManager lists the user's RSS/Atom subscriptions. New items from each
// feed land in the library as "rss" items, which never sync to Kobo devices
// (issue #640).
export default function FeedManager() {
  const { data, error, isLoading } = useFeeds()

  const feeds = data?.feeds ?? []

  return (
    <div>
      <AddFeedForm />

      {isLoading && <p className="mt-3 text-muted">Loading…</p>}
      {error && <p className="mt-3 text-danger">Failed to load feeds.</p>}
      {!isLoading && !error && feeds.length === 0 && (
        <p className="mt-3 text-sm text-muted">No feed subscriptions yet.</p>
      )}
      {feeds.length > 0 && (
        <ul className="mt-3 space-y-2">
          {feeds.map((feed) => (
            <FeedRow key={feed.id} feed={feed} />
          ))}
        </ul>
      )}
    </div>
  )
}
