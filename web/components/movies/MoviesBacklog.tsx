'use client'

import { useMemo, useState } from 'react'
import type { BacklogEntry } from '@/lib/gen/movies/v1/movies_pb'
import { useFetchMoviesBacklogPage, useMoviesActions, useMoviesBacklog } from '@/hooks/useMovies'
import { usePaginatedList } from '@/hooks/usePaginatedList'
import {
  type BacklogFilter,
  DEFAULT_BACKLOG_FILTER,
  NEW_SEASONS,
  STATUSES,
  STATUS_LABELS,
  mediaTypeLabel,
  releaseLabel,
  titleName,
  todayISO
} from '@/lib/movies/format'
import MoviePoster from '@/components/movies/MoviePoster'
import StatusSelect from '@/components/movies/StatusSelect'
import WatchedWhenDialog from '@/components/movies/WatchedWhenDialog'
import { Badge } from '@/components/ui/badge'
import { Field } from '@/components/ui/field'
import { TogglePill } from '@/components/ui/toggle-pill'
import { RatingStars } from '@/components/ui/rating-stars'
import { LinkCard } from '@/components/ui/link-card'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'
import { Select } from '@/components/ui/select'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

// "New seasons" narrows Watched, so it sits right after it.
const STATUS_TABS = [
  { value: '', label: 'All statuses' },
  ...STATUSES.flatMap((s) => [
    { value: s, label: STATUS_LABELS[s] },
    ...(s === 'watched' ? [{ value: NEW_SEASONS, label: 'New seasons' }] : [])
  ])
]

const TYPE_TABS = [
  { value: '', label: 'All types' },
  { value: 'movie', label: 'Movies' },
  { value: 'series', label: 'Series' }
]

function BacklogRow({ entry, today }: { entry: BacklogEntry; today: string }) {
  const { setStatus } = useMoviesActions()
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
      href={`/movies/${entry.id}`}
      linkClassName="flex items-center gap-3 p-3"
      actions={
        <>
          <StatusSelect
            id={`status-${entry.id}`}
            aria-label={`Status of ${displayed}`}
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
      <MoviePoster posterPath={entry.posterPath} title={displayed} />
      <div className="min-w-0">
        <p className="break-words font-medium">{displayed}</p>
        <p className="text-xs text-muted">
          {mediaTypeLabel(entry.mediaType)}
          {release && (
            <>
              {' '}
              &middot; <span suppressHydrationWarning>{release}</span>
            </>
          )}
        </p>
        <div className="flex flex-wrap items-center gap-2">
          {entry.hasNewSeason && <Badge variant="warn">New season</Badge>}
          {entry.onMyServices && <Badge variant="success">On my services</Badge>}
          {entry.rating !== undefined && <RatingStars value={entry.rating} />}
        </div>
        {failed && <p className="text-xs text-danger">Couldn&apos;t update status. Try again.</p>}
      </div>
    </LinkCard>
  )
}

export default function MoviesBacklog() {
  const [filter, setFilter] = useState<BacklogFilter>(DEFAULT_BACKLOG_FILTER)
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

  return (
    <section className="space-y-4" aria-label="Backlog">
      <SegmentedTabs
        aria-label="Filter by status"
        value={filter.status}
        options={STATUS_TABS}
        onChange={(status) => setFilter((f) => ({ ...f, status }))}
      />
      <div className="flex flex-wrap items-end gap-3">
        <SegmentedTabs
          aria-label="Filter by type"
          value={filter.mediaType}
          options={TYPE_TABS}
          onChange={(mediaType) => setFilter((f) => ({ ...f, mediaType }))}
        />
        <TogglePill
          label="On my services"
          active={filter.onMyServices}
          onClick={() => setFilter((f) => ({ ...f, onMyServices: !f.onMyServices }))}
        />
        <Field label="Sort" htmlFor="movies-sort" className="w-44">
          <Select
            id="movies-sort"
            value={filter.sort}
            onChange={(e) => setFilter((f) => ({ ...f, sort: e.target.value }))}
          >
            <option value="added">Recently added</option>
            <option value="title">Title</option>
            <option value="release">Release date</option>
            <option value="rating">Rating</option>
          </Select>
        </Field>
      </div>

      {isLoading && !data && <LoadingState label="backlog" />}
      {error && <ErrorState what="backlog" />}
      {data && entries.length === 0 && (
        <EmptyState>Nothing here yet. Search above to add a movie or series.</EmptyState>
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
