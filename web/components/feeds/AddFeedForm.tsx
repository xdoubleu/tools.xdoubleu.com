'use client'

import { useState } from 'react'
import { ConnectError, Code } from '@connectrpc/connect'
import { Button } from '@/components/ui/button'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { useCreateFeed } from '@/hooks/useFeeds'
import { buildArxivFeed } from '@/lib/feeds/arxivQuery'
import { FeedKind } from '@/lib/gen/feeds/v1/feeds_pb'

const CREATE_FEED_ERRORS: Partial<Record<Code, string>> = {
  [Code.AlreadyExists]: 'You are already subscribed to this feed.',
  [Code.InvalidArgument]: 'That URL did not work — check it and try again.',
  [Code.FailedPrecondition]: 'Email newsletters are not configured on this server.'
}

function createErrorMessage(err: unknown): string {
  if (err instanceof ConnectError) {
    return CREATE_FEED_ERRORS[err.code] ?? 'Subscribing failed. Please try again.'
  }
  return 'Subscribing failed. Please try again.'
}

type Mode = 'rss' | 'email' | 'scrape' | 'arxiv'

const MODE_KIND: Record<Mode, FeedKind> = {
  rss: FeedKind.RSS,
  email: FeedKind.EMAIL,
  scrape: FeedKind.SCRAPE,
  arxiv: FeedKind.RSS
}

function isMode(v: string): v is Mode {
  return v in MODE_KIND
}

const URL_PLACEHOLDER: Record<'rss' | 'scrape', string> = {
  rss: 'https://example.com/feed.xml',
  scrape: 'https://example.com/blog'
}

// Subscribes to an RSS/Atom feed, mints an inbound email alias for
// newsletters, scrapes a page without a feed, or follows an arXiv search.
export default function AddFeedForm({ onAdded }: { onAdded?: () => void }) {
  const createFeed = useCreateFeed()
  const [mode, setMode] = useState<Mode>('rss')
  const [url, setUrl] = useState('')
  const [title, setTitle] = useState('')
  const [arxivCategories, setArxivCategories] = useState('')
  const [arxivKeywords, setArxivKeywords] = useState('')
  const [arxivAuthor, setArxivAuthor] = useState('')
  const [busy, setBusy] = useState(false)
  const [addStatus, setAddStatus] = useState('')
  const [inboundAddress, setInboundAddress] = useState('')

  const arxivFeed = buildArxivFeed({
    categories: arxivCategories,
    keywords: arxivKeywords,
    author: arxivAuthor
  })
  const canSubmit = mode === 'email' || (mode === 'arxiv' ? arxivFeed !== null : url !== '')

  const submit = async () => {
    if (!canSubmit || busy) return
    setBusy(true)
    setAddStatus('')
    setInboundAddress('')
    try {
      const target =
        mode === 'arxiv' && arxivFeed
          ? arxivFeed
          : { url: mode === 'email' ? '' : url, title: mode === 'email' ? title.trim() : '' }
      const resp = await createFeed(target.url, MODE_KIND[mode], target.title)
      if (mode === 'email') {
        setInboundAddress(resp.feed?.inboundAddress ?? '')
      } else {
        setAddStatus('Subscribed — importing items in the background.')
      }
      setUrl('')
      setTitle('')
      setArxivCategories('')
      setArxivKeywords('')
      setArxivAuthor('')
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
          if (isMode(v)) setMode(v)
          setAddStatus('')
          setInboundAddress('')
        }}
        className="mb-2 flex-row flex-wrap gap-x-4 gap-y-0"
      >
        <RadioGroupItem value="rss" label="RSS feed" />
        <RadioGroupItem value="email" label="Email newsletter" />
        <RadioGroupItem value="scrape" label="No RSS feed" />
        <RadioGroupItem value="arxiv" label="arXiv papers" />
      </RadioGroup>

      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          void submit()
        }}
      >
        {(mode === 'rss' || mode === 'scrape') && (
          <Input
            type="url"
            required
            placeholder={URL_PLACEHOLDER[mode]}
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
        {mode === 'arxiv' && (
          <>
            <Input
              type="text"
              required
              placeholder="Categories: cs.AI, cs.LG"
              value={arxivCategories}
              onChange={(e) => setArxivCategories(e.target.value)}
              aria-label="arXiv categories"
              className="w-full sm:w-auto sm:min-w-0 sm:flex-1"
            />
            <Input
              type="text"
              placeholder="Keywords: agents, tool use"
              value={arxivKeywords}
              onChange={(e) => setArxivKeywords(e.target.value)}
              aria-label="Keywords"
              className="w-full sm:w-auto sm:min-w-0 sm:flex-1"
            />
            <Input
              type="text"
              placeholder="Author (optional)"
              value={arxivAuthor}
              onChange={(e) => setArxivAuthor(e.target.value)}
              aria-label="Author"
              className="w-full sm:w-auto sm:min-w-0 sm:flex-1"
            />
          </>
        )}
        <Button type="submit" disabled={busy || !canSubmit}>
          {busy ? 'Subscribing…' : 'Subscribe'}
        </Button>
      </form>
      {mode === 'scrape' && !addStatus && (
        <p className="mt-2 text-xs text-subtle">
          Scans the page for post links — best-effort, may miss posts on unusual layouts.
        </p>
      )}
      {mode === 'arxiv' && !addStatus && (
        <p className="mt-2 text-xs text-subtle">
          New papers in any of the categories, matching every keyword in the title or abstract. Only
          the 20 newest matches are checked each hour, so keep searches narrow.
        </p>
      )}
      {addStatus && <p className="mt-2 text-xs text-muted">{addStatus}</p>}
      {inboundAddress && (
        <Field
          className="mt-2"
          htmlFor="inbound-address"
          label="Give this address to the newsletter — save it now, it won't be shown again:"
        >
          <div className="flex items-center gap-2">
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
        </Field>
      )}
    </div>
  )
}
