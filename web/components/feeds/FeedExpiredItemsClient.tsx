import { useCallback, useMemo, useState, type Dispatch } from 'react'
import { useFeeds } from '@/hooks/useFeeds'
import { useExpiredFeedItems, useFetchExpiredFeedItemsPage } from '@/hooks/useFeedExpiry'
import { useRestoreFeedItem } from '@/hooks/useFeedFilteredItems'
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
import type { Item } from '@/lib/gen/feeds/v1/feeds_pb'

// Named type, not an inline function type: StrykerJS's instrumenter crashes
// printing a TS function type.
interface ExpiredItemRowProps {
  item: Item
  feedTitles: ReadonlyMap<string, string>
  onRestored: Dispatch<Item>
}

function ExpiredItemRow({ item, feedTitles, onRestored }: ExpiredItemRowProps) {
  const restore = useRestoreFeedItem()
  const [busy, setBusy] = useState(false)
  const [failed, setFailed] = useState(false)

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
          <p className="text-xs text-muted wrap-anywhere">
            {feedTitles.get(item.feedId) ?? 'Unknown feed'}
          </p>
          <FeedItemCategories categories={item.categories} />
          <p className="text-xs text-subtle">
            Arrived {formatDate(item.createdAt)} · expired {formatDate(item.expiredAt)}
          </p>
          {failed && (
            <p className="text-xs text-danger" role="alert">
              Restoring failed. Please try again.
            </p>
          )}
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

// Items that expired unread, each with a Restore action.
export default function FeedExpiredItemsClient({ initialFeedId }: { initialFeedId?: string }) {
  const [feedId, setFeedId] = useState<string | undefined>(initialFeedId || undefined)
  const { data: feedsData } = useFeeds()
  const { data, error, isLoading } = useExpiredFeedItems(feedId)
  const fetchPage = useFetchExpiredFeedItemsPage(feedId)
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
  const feedTitles = useMemo(() => new Map(feeds.map((f) => [f.id, feedLabel(f)])), [feeds])
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

      {isLoading && <LoadingState label="expired items" />}
      {error && <ErrorState what="expired items" />}
      {!isLoading && !error && visible.length === 0 && <EmptyState>No expired items.</EmptyState>}
      {visible.length > 0 && (
        <ul className="space-y-2">
          {visible.map((item) => (
            <ExpiredItemRow
              key={item.id}
              item={item}
              feedTitles={feedTitles}
              onRestored={handleRestored}
            />
          ))}
        </ul>
      )}
      {hasMore && <LoadMoreButton onClick={loadMore} loading={loading} />}
    </div>
  )
}
