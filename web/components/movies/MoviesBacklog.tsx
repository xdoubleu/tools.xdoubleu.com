import { useEffect, useMemo, useState } from 'react'
import type { BacklogEntry } from '@/lib/gen/movies/v1/movies_pb'
import { useFetchMoviesBacklogPage, useMoviesActions, useMoviesBacklog } from '@/hooks/useMovies'
import { useSharedList } from '@/hooks/useSharedList'
import { listHref } from '@/lib/listScope'
import { usePaginatedList } from '@/hooks/usePaginatedList'
import {
  type BacklogFilter,
  mediaTypeLabel,
  posterUrl,
  releaseLabel,
  titleName,
  todayISO
} from '@/lib/movies/format'
import { Artwork } from '@/components/ui/artwork'
import StatusSelect from '@/components/movies/StatusSelect'
import WatchedWhenDialog from '@/components/movies/WatchedWhenDialog'
import { Badge } from '@/components/ui/badge'
import { FieldError } from '@/components/ui/field-error'
import { RatingStars } from '@/components/ui/rating-stars'
import { LinkCard } from '@/components/ui/link-card'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'
import { Button } from '@/components/ui/button'
import { focusById } from '@/lib/focusById'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

function BacklogRow({ entry, today }: { entry: BacklogEntry; today: string }) {
  const { setStatus } = useMoviesActions()
  const shared = useSharedList()
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)
  // Rows past the first page aren't refetched, so a saved status is held
  // locally until the entry itself changes.
  const [saved, setSaved] = useState<{ from: string; to: string } | null>(null)
  const [asking, setAsking] = useState(false)
  const status = saved?.from === entry.status ? saved.to : entry.status
  const release = releaseLabel(entry.releaseDate, today)
  const displayed = titleName(entry)

  const change = async (next: string, unknownDate = false) => {
    setPending(true)
    setFailed(false)
    try {
      await setStatus(entry.id, next, unknownDate)
      setSaved({ from: entry.status, to: next })
    } catch {
      setFailed(true)
    } finally {
      setPending(false)
      setAsking(false)
    }
  }

  // Marking a multi-season series (only series have a season count) watched
  // asks when, so old binges don't date as today.
  const select = (next: string) => {
    if (next === 'watched' && (entry.seasonCount ?? 0) > 1) {
      setAsking(true)
    } else {
      void change(next)
    }
  }

  return (
    <LinkCard
      href={listHref(`/movies/${entry.id}`, shared)}
      linkClassName="flex items-center gap-3 p-3"
      actions={
        <>
          <StatusSelect
            id={`status-${entry.id}`}
            aria-label={`Status of ${displayed}`}
            aria-describedby={failed ? `status-${entry.id}-error` : undefined}
            value={status}
            disabled={pending}
            onChange={select}
          />
          <WatchedWhenDialog
            open={asking}
            onOpenChange={setAsking}
            title={displayed}
            pending={pending}
            onChoose={(unknownDate) => void change('watched', unknownDate)}
          />
        </>
      }
    >
      <Artwork src={posterUrl(entry.posterPath, 'w92')} title={displayed} size="md" />
      <div className="min-w-0">
        <p className="break-words font-medium">{displayed}</p>
        <p className="text-xs text-muted">
          {mediaTypeLabel(entry.mediaType)}
          {release && (
            <>
              {' '}
              &middot; <span>{release}</span>
            </>
          )}
        </p>
        <div className="flex flex-wrap items-center gap-2">
          {entry.hasNewSeason && <Badge variant="warn">New season</Badge>}
          {entry.onMyServices && <Badge variant="success">On my services</Badge>}
          {entry.rating !== undefined && <RatingStars value={entry.rating} />}
        </div>
        <FieldError id={`status-${entry.id}-error`}>
          {failed && 'Couldn’t update status. Try again.'}
        </FieldError>
      </div>
    </LinkCard>
  )
}

interface MoviesBacklogProps {
  filter: BacklogFilter
  /** Receives the loaded-entries label for the toolbar. */
  onCount?: (label: string | null) => void
}

export default function MoviesBacklog({ filter, onCount }: MoviesBacklogProps) {
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
  const countLabel = data
    ? `${entries.length}${hasMore ? '+' : ''} ${entries.length === 1 && !hasMore ? 'title' : 'titles'}`
    : null

  useEffect(() => {
    onCount?.(countLabel)
  }, [onCount, countLabel])

  return (
    <section className="space-y-4" aria-label="Backlog">
      {isLoading && !data && <LoadingState placement="page" label="backlog" />}
      {error && <ErrorState placement="page" what="backlog" />}
      {data && entries.length === 0 && (
        <EmptyState
          placement="page"
          action={
            <Button size="sm" onClick={() => focusById('movies-search')}>
              Add a movie or series
            </Button>
          }
        >
          Nothing here yet. Search TMDB to add your first title.
        </EmptyState>
      )}
      {entries.length > 0 && (
        <>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {entries.map((e) => (
              <BacklogRow key={e.id} entry={e} today={today} />
            ))}
          </div>
          {hasMore && <LoadMoreButton onClick={loadMore} loading={loadingMore} />}
        </>
      )}
    </section>
  )
}
