'use client'

import { useEffect, useRef, useState } from 'react'
import { useSearchLibrary } from '@/hooks/useBooks'
import { useFeedItems } from '@/hooks/useFeeds'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { MenuItem } from '@/components/ui/menu-item'

// ResourceLinkPicker lets a learning path resource attach an existing books
// library entry or feeds item alongside its freeform text (#1474). Search
// results here are only ever the caller's own data (useSearchLibrary /
// useFeedItems are both scoped server-side to the authenticated user), so
// picking a result is guaranteed to resolve for Create/UpdateLearningPath's
// server-side link validation.
interface LinkedBookValue {
  id: string
  title: string
}

interface LinkedFeedItemValue {
  id: string
  title: string
}

interface ResourceLinkPickerProps {
  linkedBook?: LinkedBookValue
  linkedFeedItem?: LinkedFeedItemValue
  onLinkBook: (value: LinkedBookValue) => void
  onLinkFeedItem: (value: LinkedFeedItemValue) => void
  onUnlink: () => void
}

export default function ResourceLinkPicker({
  linkedBook,
  linkedFeedItem,
  onLinkBook,
  onLinkFeedItem,
  onUnlink
}: ResourceLinkPickerProps) {
  const [mode, setMode] = useState<'none' | 'book' | 'feed'>('none')

  if (linkedBook) {
    return (
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <span className="max-w-full break-words rounded-full bg-accent/10 px-2.5 py-1 text-accent">
          📚 {linkedBook.title}
        </span>
        <Button type="button" variant="ghost" size="sm" onClick={onUnlink}>
          Unlink
        </Button>
      </div>
    )
  }

  if (linkedFeedItem) {
    return (
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <span className="max-w-full break-words rounded-full bg-accent/10 px-2.5 py-1 text-accent">
          📰 {linkedFeedItem.title}
        </span>
        <Button type="button" variant="ghost" size="sm" onClick={onUnlink}>
          Unlink
        </Button>
      </div>
    )
  }

  if (mode === 'book') {
    return (
      <BookLinkSearch
        onPick={(value) => {
          onLinkBook(value)
          setMode('none')
        }}
        onCancel={() => setMode('none')}
      />
    )
  }

  if (mode === 'feed') {
    return (
      <FeedItemLinkSearch
        onPick={(value) => {
          onLinkFeedItem(value)
          setMode('none')
        }}
        onCancel={() => setMode('none')}
      />
    )
  }

  return (
    <div className="flex gap-2">
      <Button type="button" variant="secondary" size="sm" onClick={() => setMode('book')}>
        Link a book
      </Button>
      <Button type="button" variant="secondary" size="sm" onClick={() => setMode('feed')}>
        Link a feed item
      </Button>
    </div>
  )
}

function BookLinkSearch({
  onPick,
  onCancel
}: {
  onPick: (value: LinkedBookValue) => void
  onCancel: () => void
}) {
  const searchLibrary = useSearchLibrary()
  const [query, setQuery] = useState('')
  const [hits, setHits] = useState<{ id: string; title: string }[]>([])
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (debounceTimer.current) clearTimeout(debounceTimer.current)
    if (!query.trim()) {
      setHits([])
      return
    }
    debounceTimer.current = setTimeout(async () => {
      try {
        const resp = await searchLibrary(query.trim())
        setHits(resp.books.map((ub) => ({ id: ub.bookId, title: ub.book?.title ?? '(untitled)' })))
      } catch {
        setHits([])
      }
    }, 300)
    return () => {
      if (debounceTimer.current) clearTimeout(debounceTimer.current)
    }
  }, [query, searchLibrary])

  return (
    <div className="space-y-1.5">
      <div className="flex gap-2">
        <Input
          type="text"
          autoFocus
          placeholder="Search your library…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="flex-1"
        />
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
      {hits.length > 0 && (
        <ul className="max-h-40 overflow-y-auto rounded-2xl border border-border bg-card shadow-elevated">
          {hits.map((hit) => (
            <li key={hit.id}>
              <MenuItem onClick={() => onPick(hit)}>{hit.title}</MenuItem>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function FeedItemLinkSearch({
  onPick,
  onCancel
}: {
  onPick: (value: LinkedFeedItemValue) => void
  onCancel: () => void
}) {
  // No server-side text search for feed items — filter the already-cached
  // full list client-side, matching feed item counts small enough this
  // stays instant.
  const { data } = useFeedItems(false)
  const [query, setQuery] = useState('')

  const hits = (data?.items ?? [])
    .filter((item) => item.title.toLowerCase().includes(query.trim().toLowerCase()))
    .slice(0, 8)

  return (
    <div className="space-y-1.5">
      <div className="flex gap-2">
        <Input
          type="text"
          autoFocus
          placeholder="Search your feed items…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="flex-1"
        />
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
      {query.trim() && hits.length > 0 && (
        <ul className="max-h-40 overflow-y-auto rounded-2xl border border-border bg-card shadow-elevated">
          {hits.map((item) => (
            <li key={item.id}>
              <MenuItem onClick={() => onPick({ id: item.id, title: item.title })}>
                {item.title}
              </MenuItem>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
