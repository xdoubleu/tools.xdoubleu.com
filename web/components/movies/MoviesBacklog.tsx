import { type ReactNode, useMemo, useState } from 'react'
import { Clapperboard } from 'lucide-react'
import type { BacklogEntry } from '@/lib/gen/movies/v1/movies_pb'
import { useFetchMoviesBacklogPage, useMoviesBacklog } from '@/hooks/useMovies'
import { useSharedList } from '@/hooks/useSharedList'
import { listHref } from '@/lib/listScope'
import { usePaginatedList } from '@/hooks/usePaginatedList'
import {
  type BacklogFilter,
  hasActiveFilter,
  mediaTypeLabel,
  posterUrl,
  releaseLabel,
  statusLabel,
  titleName,
  todayISO
} from '@/lib/movies/format'
import { Artwork } from '@/components/ui/artwork'
import MoviesBacklogFilters from '@/components/movies/MoviesBacklogFilters'
import { Badge } from '@/components/ui/badge'
import { RatingStars } from '@/components/ui/rating-stars'
import { CollectionGrid } from '@/components/ui/collection-grid'
import { CollectionToolbar } from '@/components/ui/collection-toolbar'
import { ItemCard } from '@/components/ui/item-card'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

function BacklogCard({ entry, today }: { entry: BacklogEntry; today: string }) {
  const shared = useSharedList()
  const release = releaseLabel(entry.releaseDate, today)
  const displayed = titleName(entry)

  return (
    <ItemCard
      href={listHref(`/movies/${entry.id}`, shared)}
      title={displayed}
      artwork={<Artwork src={posterUrl(entry.posterPath, 'w92')} title={displayed} size="md" />}
      meta={
        <>
          {mediaTypeLabel(entry.mediaType)}
          {release && <> &middot; {release}</>}
        </>
      }
      status={
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="secondary">{statusLabel(entry.status)}</Badge>
          {entry.hasNewSeason && <Badge variant="warn">New season</Badge>}
          {entry.onMyServices && <Badge variant="success">On my services</Badge>}
          {entry.rating !== undefined && <RatingStars value={entry.rating} />}
        </div>
      }
    />
  )
}

interface MoviesBacklogProps {
  filter: BacklogFilter
  onFilterChange: (filter: BacklogFilter) => void
  /** Opens the add-a-title search, offered when the backlog is empty. */
  addAction: ReactNode
}

export default function MoviesBacklog({ filter, onFilterChange, addAction }: MoviesBacklogProps) {
  const { data, error, isLoading } = useMoviesBacklog(filter)
  const fetchPage = useFetchMoviesBacklogPage(filter)
  const initialPage = useMemo(
    // Stryker disable next-line BooleanLiteral: without data there are no entries to load more of.
    () => ({ items: data?.entries ?? [], hasMore: data?.hasMore ?? false }),
    [data]
  )
  const {
    items: entries,
    hasMore,
    loading: loadingMore,
    loadMore
  } = usePaginatedList(initialPage, fetchPage, (a, b) => a.id === b.id)
  const today = todayISO()
  const [query, setQuery] = useState('')
  const shown = useMemo(() => {
    const needle = query.trim().toLowerCase()
    return entries.filter((e) => titleName(e).toLowerCase().includes(needle))
  }, [entries, query])

  return (
    <section className="space-y-4" aria-label="Backlog">
      {isLoading && !data && <LoadingState placement="page" label="backlog" />}
      {error && <ErrorState placement="page" what="backlog" />}
      {data && entries.length === 0 && !hasActiveFilter(filter) && (
        <EmptyState placement="page" icon={Clapperboard} action={addAction}>
          Nothing here yet. Add your first title.
        </EmptyState>
      )}
      {data && (entries.length > 0 || hasActiveFilter(filter)) && (
        <>
          <CollectionToolbar
            noun="titles"
            query={query}
            onQueryChange={setQuery}
            count={shown.length}
          >
            <MoviesBacklogFilters filter={filter} onChange={onFilterChange} />
          </CollectionToolbar>
          {shown.length === 0 ? (
            <EmptyState placement="inline">No titles match.</EmptyState>
          ) : (
            <CollectionGrid>
              {shown.map((e) => (
                <BacklogCard key={e.id} entry={e} today={today} />
              ))}
            </CollectionGrid>
          )}
          {hasMore && <LoadMoreButton onClick={loadMore} loading={loadingMore} />}
        </>
      )}
    </section>
  )
}
