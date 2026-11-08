import { useEffect, useState } from 'react'
import ListSwitch from '@/components/ListSwitch'
import MovieSearchResults from '@/components/movies/MovieSearchResults'
import MoviesBacklog from '@/components/movies/MoviesBacklog'
import MoviesBacklogFilters from '@/components/movies/MoviesBacklogFilters'
import TmdbAttribution from '@/components/movies/TmdbAttribution'
import { useSharedList } from '@/hooks/useSharedList'
import { listHref } from '@/lib/listScope'
import { type BacklogFilter, DEFAULT_BACKLOG_FILTER } from '@/lib/movies/format'
import { Input } from '@/components/ui/input'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderLink, PageHeaderSettingsLink } from '@/components/ui/page-header'
import { Toolbar } from '@/components/ui/toolbar'

// Each search is a TMDB request; wait for typing to pause.
const SEARCH_DEBOUNCE_MS = 300

/** Backlog by default; a two-character search swaps in TMDB results. */
export default function MoviesClient() {
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [filter, setFilter] = useState<BacklogFilter>(DEFAULT_BACKLOG_FILTER)
  const [count, setCount] = useState<string | null>(null)
  const shared = useSharedList()
  const searching = debouncedQuery.trim().length >= 2

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [query])

  return (
    <PageContainer>
      <PageHeader
        title="Movies & series"
        actions={
          <>
            <PageHeaderLink href={listHref('/movies/stats', shared)}>Stats</PageHeaderLink>
            <PageHeaderSettingsLink href="/movies/settings" />
          </>
        }
      />
      <ListSwitch className="mb-4" />
      <Toolbar
        aria-label="Filter movies and series"
        search={
          <Input
            id="movies-search"
            type="search"
            aria-label="Search TMDB"
            placeholder="Search TMDB for a movie or series…"
            autoComplete="off"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        }
        trailing={count ?? undefined}
      >
        {!searching && <MoviesBacklogFilters filter={filter} onChange={setFilter} />}
      </Toolbar>
      {searching ? (
        <MovieSearchResults query={debouncedQuery} onCount={setCount} />
      ) : (
        <MoviesBacklog filter={filter} onCount={setCount} />
      )}
      <TmdbAttribution />
    </PageContainer>
  )
}
