'use client'

import { useState } from 'react'
import { ConnectError, Code } from '@connectrpc/connect'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { useCreateFeed } from '@/hooks/useBookFeeds'

const CREATE_FEED_ERRORS: Partial<Record<Code, string>> = {
  [Code.AlreadyExists]: 'You are already subscribed to this feed.',
  [Code.InvalidArgument]: 'That URL is not a valid RSS/Atom feed.'
}

function createErrorMessage(err: unknown): string {
  if (err instanceof ConnectError) {
    return CREATE_FEED_ERRORS[err.code] ?? 'Subscribing failed. Please try again.'
  }
  return 'Subscribing failed. Please try again.'
}

// AddFeedForm subscribes to an RSS/Atom feed. New items from the feed land in
// the library as "rss" items; the Kobo-sync checkbox auto-opts new items into
// Kobo syncing. Shared by the settings FeedManager and the unified add dialog.
export default function AddFeedForm({ onAdded }: { onAdded?: () => void }) {
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
      await createFeed(url.trim(), koboSync)
      setAddStatus('Subscribed — importing items in the background.')
      setUrl('')
      setKoboSync(false)
      onAdded?.()
    } catch (err) {
      setAddStatus(createErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

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
    </div>
  )
}
