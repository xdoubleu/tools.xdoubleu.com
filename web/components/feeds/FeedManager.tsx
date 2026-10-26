import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { ConfirmDialog } from '@/components/ui/dialog'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import AddFeedForm from '@/components/feeds/AddFeedForm'
import { useFeeds, useDeleteFeed, useRefreshFeed } from '@/hooks/useFeeds'
import type { Feed } from '@/lib/gen/feeds/v1/feeds_pb'

function FeedRow({ feed }: { feed: Feed }) {
  const deleteFeed = useDeleteFeed()
  const refreshFeed = useRefreshFeed()
  const [busy, setBusy] = useState(false)
  const [status, setStatus] = useState('')
  const [confirming, setConfirming] = useState(false)

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
  const name = feed.title || feed.url || 'Email newsletter'

  return (
    <li>
      <Card className="p-3">
        <div className="flex flex-wrap items-center gap-2">
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">{name}</p>
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
            onClick={() => setConfirming(true)}
          >
            Remove
          </Button>
        </div>
        <div className="mt-1 space-y-0.5">
          {feed.lastError && (
            // eslint-disable-next-line ui/use-primitives -- the last poll's stored error, not a live failure
            <p className="break-words text-xs text-danger">
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
              Waiting for your first email — give the address above to the newsletter, then check
              back here.
            </p>
          )}
          {status && <p className="text-xs text-muted">{status}</p>}
        </div>
      </Card>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={`Remove ${name}?`}
        description="Its items, bookmarks and filter rules are deleted too."
        confirmLabel="Remove"
        pendingLabel="Removing…"
        destructive
        pending={busy}
        onConfirm={() => void run(() => deleteFeed(feed.id)).then(() => setConfirming(false))}
      />
    </li>
  )
}

export default function FeedManager() {
  const { data, error, isLoading } = useFeeds()

  const feeds = data?.feeds ?? []

  return (
    <div>
      <AddFeedForm />

      {isLoading && <LoadingState className="mt-3" />}
      {error && <ErrorState what="feeds" className="mt-3" />}
      {!isLoading && !error && feeds.length === 0 && (
        <EmptyState className="mt-3">No feed subscriptions yet.</EmptyState>
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
