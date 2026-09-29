'use client'

import { useCallback, useMemo, useState } from 'react'
import { useFeeds } from '@/hooks/useFeeds'
import {
  useFetchFilteredFeedItemsPage,
  useFilteredFeedItems,
  useRestoreFeedItem
} from '@/hooks/useFeedFilteredItems'
import { usePaginatedList } from '@/hooks/usePaginatedList'
import FeedItemCategories from '@/components/feeds/FeedItemCategories'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'
import { Select } from '@/components/ui/select'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { formatDate } from '@/lib/dates'
import { feedLabel } from '@/lib/feeds/feedLabel'
import { FilterRuleKind } from '@/lib/gen/feeds/v1/feeds_pb'
import type { FilterRule, Item } from '@/lib/gen/feeds/v1/feeds_pb'

function FilterReason({ rule, scope }: { rule?: FilterRule; scope: string }) {
  if (!rule) return <>Rule deleted</>
  const kind = rule.kind === FilterRuleKind.CATEGORY ? 'Category is' : 'Title contains'
  return (
    <>
      {kind} <em>{rule.value}</em> · {scope}
    </>
  )
}

interface FilteredItemRowProps {
  item: Item
  feedTitle: (feedId: string) => string
  onRestored: (item: Item) => void
}

function FilteredItemRow({ item, feedTitle, onRestored }: FilteredItemRowProps) {
  const restore = useRestoreFeedItem()
  const [busy, setBusy] = useState(false)
  const [failed, setFailed] = useState(false)
  const rule = item.filterRule
  const scope = rule?.feedId ? feedTitle(rule.feedId) : 'All feeds'

  const submit = async () => {
    setBusy(true)
    setFailed(false)
    try {
      await restore(item.id)
      onRestored(item)
    } catch {
      setFailed(true)
      setBusy(false)
    }
  }

  return (
    <li>
      <Card className="flex flex-col gap-2 p-3 sm:flex-row sm:items-start">
        <div className="min-w-0 flex-1 space-y-1">
          <Button
            asChild
            variant="link"
            className="h-auto min-w-0 justify-start p-0 text-left text-sm font-semibold text-fg wrap-anywhere hover:text-accent"
          >
            <a href={item.sourceUrl} target="_blank" rel="noopener noreferrer">
              {item.title}
            </a>
          </Button>
          <p className="text-xs text-muted wrap-anywhere">{feedTitle(item.feedId)}</p>
          <FeedItemCategories categories={item.categories} />
          <p className="text-xs text-muted wrap-anywhere">
            <FilterReason rule={rule} scope={scope} />
          </p>
          <p className="text-xs text-subtle">Filtered {formatDate(item.filteredAt)}</p>
          {failed && <p className="text-xs text-danger">Restoring failed. Please try again.</p>}
        </div>
        <Button
          size="sm"
          variant="secondary"
          disabled={busy}
          aria-label={busy ? undefined : `Restore ${item.title}`}
          onClick={() => void submit()}
        >
          {busy ? 'Restoring…' : 'Restore'}
        </Button>
      </Card>
    </li>
  )
}

// Items filter rules hid, each with the matching rule and a Restore action.
export default function FeedFilteredItemsClient({ initialFeedId }: { initialFeedId?: string }) {
  const [feedId, setFeedId] = useState<string | undefined>(initialFeedId || undefined)
  const { data: feedsData } = useFeeds()
  const { data, error, isLoading } = useFilteredFeedItems(feedId)
  const fetchPage = useFetchFilteredFeedItemsPage(feedId)
  const initialPage = useMemo(
    () => ({ items: data?.items ?? [], hasMore: data?.hasMore ?? false }),
    [data]
  )
  const { items, hasMore, loading, loadMore } = usePaginatedList(
    initialPage,
    fetchPage,
    (a, b) => a.id === b.id
  )
  // Hidden at once; the refetch that drops them may still be in flight.
  const [restoredIds, setRestoredIds] = useState<Set<string>>(new Set())
  const [restoredTitle, setRestoredTitle] = useState<string | null>(null)

  const feeds = useMemo(() => feedsData?.feeds ?? [], [feedsData])
  const feedTitle = useCallback(
    (id: string) => {
      const feed = feeds.find((f) => f.id === id)
      return feed ? feedLabel(feed) : 'Unknown feed'
    },
    [feeds]
  )
  const handleRestored = useCallback((item: Item) => {
    setRestoredIds((prev) => new Set(prev).add(item.id))
    setRestoredTitle(item.title)
  }, [])

  const visible = items.filter((item) => !restoredIds.has(item.id))

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Select
          value={feedId ?? ''}
          onChange={(e) => setFeedId(e.target.value || undefined)}
          aria-label="Filter by feed"
          className="w-full sm:w-auto"
        >
          <option value="">All feeds</option>
          {feeds.map((feed) => (
            <option key={feed.id} value={feed.id}>
              {feedLabel(feed)}
            </option>
          ))}
        </Select>
      </div>

      {restoredTitle && <Alert tone="success">Restored “{restoredTitle}” to the inbox.</Alert>}

      {isLoading && <LoadingState label="filtered items" />}
      {error && <ErrorState what="filtered items" />}
      {!isLoading && !error && visible.length === 0 && <EmptyState>No filtered items.</EmptyState>}
      {visible.length > 0 && (
        <ul className="space-y-2">
          {visible.map((item) => (
            <FilteredItemRow
              key={item.id}
              item={item}
              feedTitle={feedTitle}
              onRestored={handleRestored}
            />
          ))}
        </ul>
      )}
      {hasMore && <LoadMoreButton onClick={loadMore} loading={loading} />}
    </div>
  )
}
