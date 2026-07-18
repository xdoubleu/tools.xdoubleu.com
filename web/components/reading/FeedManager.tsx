'use client'

import { useState } from 'react'
import { ConnectError, Code } from '@connectrpc/connect'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import {
  useFeeds,
  useCreateFeed,
  useUpdateFeed,
  useDeleteFeed,
  useRefreshFeed
} from '@/hooks/useBookFeeds'
import type { Feed } from '@/lib/gen/reading/v1/feeds_pb'

function createErrorMessage(err: unknown): string {
  if (err instanceof ConnectError) {
    if (err.code === Code.AlreadyExists) return 'You are already subscribed to this feed.'
    if (err.code === Code.InvalidArgument) return 'That URL is not a valid RSS/Atom feed.'
  }
  return 'Subscribing failed. Please try again.'
}

function FeedRow({ feed }: { feed: Feed }) {
  const updateFeed = useUpdateFeed()
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

  return (
    <li className="rounded-2xl border border-border bg-card p-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium">{feed.title || feed.url}</p>
          <p className="truncate text-xs text-muted">{feed.url}</p>
        </div>
        <label className="flex items-center gap-1.5 text-xs text-subtle">
          <Checkbox
            checked={feed.koboSync}
            disabled={busy}
            onChange={(e) => void run(() => updateFeed(feed.id, feed.title, e.target.checked))}
          />
          Kobo sync
        </label>
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
          <p className="text-xs text-danger">Last poll failed: {feed.lastError}</p>
        )}
        {feed.lastFetchedAt && !feed.lastError && (
          <p className="text-xs text-muted">
            Last fetched {new Date(feed.lastFetchedAt).toLocaleString()}
          </p>
        )}
        {status && <p className="text-xs text-muted">{status}</p>}
      </div>
    </li>
  )
}

// FeedManager lists the user's RSS/Atom subscriptions. New items from each
// feed land in the library as "rss" items; the per-feed Kobo sync checkbox
// auto-opts new items into Kobo syncing.
export default function FeedManager() {
  const { data, error, isLoading } = useFeeds()
  const createFeed = useCreateFeed()
  const [url, setUrl] = useState('')
  const [koboSync, setKoboSync] = useState(false)
  const [busy, setBusy] = useState(false)
  const [addStatus, setAddStatus] = useState('')

  const submit = async () => {
    if (!url.trim() || busy) return
    setBusy(true)
    setAddStatus('')
    try {
      const resp = await createFeed(url.trim(), koboSync)
      setAddStatus(`Subscribed — imported ${resp.ingested} item(s).`)
      setUrl('')
      setKoboSync(false)
    } catch (err) {
      setAddStatus(createErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const feeds = data?.feeds ?? []

  return (
    <div>
      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          void submit()
        }}
      >
        <Input
          type="url"
          required
          placeholder="https://example.com/feed.xml"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          aria-label="Feed URL"
          className="w-auto min-w-0 flex-1"
        />
        <label className="flex items-center gap-1.5 text-xs text-subtle">
          <Checkbox checked={koboSync} onChange={(e) => setKoboSync(e.target.checked)} />
          Kobo sync
        </label>
        <Button type="submit" disabled={busy || !url.trim()}>
          {busy ? 'Subscribing…' : 'Subscribe'}
        </Button>
      </form>
      {addStatus && <p className="mt-2 text-xs text-muted">{addStatus}</p>}

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
