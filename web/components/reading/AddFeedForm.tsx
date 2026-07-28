'use client'

import { useState } from 'react'
import { ConnectError, Code } from '@connectrpc/connect'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { useCreateFeed } from '@/hooks/useBookFeeds'
import { FeedKind } from '@/lib/gen/reading/v1/feeds_pb'

const CREATE_FEED_ERRORS: Partial<Record<Code, string>> = {
  [Code.AlreadyExists]: 'You are already subscribed to this feed.',
  [Code.InvalidArgument]: 'That URL is not a valid RSS/Atom feed.',
  [Code.FailedPrecondition]: 'Email newsletters are not configured on this server.'
}

function createErrorMessage(err: unknown): string {
  if (err instanceof ConnectError) {
    return CREATE_FEED_ERRORS[err.code] ?? 'Subscribing failed. Please try again.'
  }
  return 'Subscribing failed. Please try again.'
}

type Mode = 'rss' | 'email'

// AddFeedForm subscribes to an RSS/Atom feed, or mints a per-feed inbound
// email alias for newsletters with no public feed (issue #595). New items
// land in the library as "rss" items either way; RSS items never sync to
// Kobo devices (issue #640). Shared by the /feeds page's FeedManager and the
// unified add dialog.
export default function AddFeedForm({ onAdded }: { onAdded?: () => void }) {
  const createFeed = useCreateFeed()
  const [mode, setMode] = useState<Mode>('rss')
  const [url, setUrl] = useState('')
  const [title, setTitle] = useState('')
  const [busy, setBusy] = useState(false)
  const [addStatus, setAddStatus] = useState('')
  const [inboundAddress, setInboundAddress] = useState('')

  const submit = async () => {
    if ((mode === 'rss' && !url.trim()) || busy) return
    setBusy(true)
    setAddStatus('')
    setInboundAddress('')
    try {
      const resp = await createFeed(
        mode === 'rss' ? url.trim() : '',
        mode === 'rss' ? FeedKind.RSS : FeedKind.EMAIL,
        mode === 'email' ? title.trim() : ''
      )
      if (mode === 'email') {
        setInboundAddress(resp.feed?.inboundAddress ?? '')
      } else {
        setAddStatus('Subscribed — importing items in the background.')
      }
      setUrl('')
      setTitle('')
      onAdded?.()
    } catch (err) {
      setAddStatus(createErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <RadioGroup
        name="feed-mode"
        value={mode}
        onChange={(v) => {
          setMode(v === 'email' ? 'email' : 'rss')
          setAddStatus('')
          setInboundAddress('')
        }}
        className="mb-2 flex-row gap-4"
      >
        <RadioGroupItem value="rss" label="RSS feed" />
        <RadioGroupItem value="email" label="Email newsletter" />
      </RadioGroup>

      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          void submit()
        }}
      >
        {mode === 'rss' && (
          <Input
            type="url"
            required
            placeholder="https://example.com/feed.xml"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            aria-label="Feed URL"
            className="w-auto min-w-0 flex-1"
          />
        )}
        {mode === 'email' && (
          <Input
            type="text"
            placeholder="Newsletter name (optional)"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            aria-label="Newsletter name"
            className="w-auto min-w-0 flex-1"
          />
        )}
        <Button type="submit" disabled={busy || (mode === 'rss' && !url.trim())}>
          {busy ? 'Subscribing…' : 'Subscribe'}
        </Button>
      </form>
      {addStatus && <p className="mt-2 text-xs text-muted">{addStatus}</p>}
      {inboundAddress && (
        <div className="mt-2">
          <label className="text-xs text-subtle" htmlFor="inbound-address">
            Give this address to the newsletter — save it now, it won&apos;t be shown again:
          </label>
          <div className="mt-1 flex items-center gap-2">
            <Input id="inbound-address" readOnly value={inboundAddress} className="flex-1" />
            <Button
              type="button"
              size="sm"
              variant="secondary"
              onClick={() => void navigator.clipboard.writeText(inboundAddress)}
            >
              Copy
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
