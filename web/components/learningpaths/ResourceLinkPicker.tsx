'use client'

import { type ReactNode, useEffect, useRef, useState } from 'react'
import { useSearchLibrary } from '@/hooks/useBooks'
import { useFeedItems } from '@/hooks/useFeeds'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Combobox } from '@/components/ui/combobox'

// Attaches a library book or feed item to a resource. Results come only from
// the caller's own data, so they pass the server's link validation.
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
        <Badge className="max-w-full break-words text-sm">📚 {linkedBook.title}</Badge>
        <Button type="button" variant="ghost" size="sm" onClick={onUnlink}>
          Unlink
        </Button>
      </div>
    )
  }

  if (linkedFeedItem) {
    return (
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <Badge className="max-w-full break-words text-sm">📰 {linkedFeedItem.title}</Badge>
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
    <SearchRow onCancel={onCancel}>
      <Combobox
        autoFocus
        placeholder="Search your library…"
        aria-label="Search your library"
        value={query}
        onChange={setQuery}
        suggestions={hits.map((hit) => hit.title)}
        filterSuggestions={false}
        onSelect={(title) =>
          hits
            .filter((h) => h.title === title)
            .slice(0, 1)
            .forEach(onPick)
        }
        className="min-w-0 flex-1"
      />
    </SearchRow>
  )
}

function FeedItemLinkSearch({
  onPick,
  onCancel
}: {
  onPick: (value: LinkedFeedItemValue) => void
  onCancel: () => void
}) {
  // No server-side feed item search; filter the cached list client-side.
  const { data } = useFeedItems(false)
  const [query, setQuery] = useState('')

  const hits = (data?.items ?? [])
    .filter((item) => item.title.toLowerCase().includes(query.trim().toLowerCase()))
    .slice(0, 8)

  return (
    <SearchRow onCancel={onCancel}>
      <Combobox
        autoFocus
        placeholder="Search your feed items…"
        aria-label="Search your feed items"
        value={query}
        onChange={setQuery}
        suggestions={query.trim() ? hits.map((item) => item.title) : []}
        filterSuggestions={false}
        onSelect={(title) =>
          hits
            .filter((h) => h.title === title)
            .slice(0, 1)
            .forEach((item) => onPick({ id: item.id, title: item.title }))
        }
        className="min-w-0 flex-1"
      />
    </SearchRow>
  )
}

function SearchRow({ onCancel, children }: { onCancel: () => void; children: ReactNode }) {
  return (
    <div className="flex gap-2">
      {children}
      <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
        Cancel
      </Button>
    </div>
  )
}
