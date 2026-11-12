import { useEffect, useState } from 'react'
import ListSwitch from '@/components/ListSwitch'
import MovieSearchResults from '@/components/movies/MovieSearchResults'
import MoviesBacklog from '@/components/movies/MoviesBacklog'
import TmdbAttribution from '@/components/movies/TmdbAttribution'
import { useSharedList } from '@/hooks/useSharedList'
import { listHref } from '@/lib/listScope'
import { type BacklogFilter, DEFAULT_BACKLOG_FILTER } from '@/lib/movies/format'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderLink, PageHeaderSettingsLink } from '@/components/ui/page-header'

// Each search is a TMDB request; wait for typing to pause.
const SEARCH_DEBOUNCE_MS = 300

/** TMDB search in a sheet; a two-character query fetches results. */
function AddTitleDialog({
  open,
  onOpenChange
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const searching = debouncedQuery.trim().length >= 2

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [query])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add a title</DialogTitle>
        </DialogHeader>
        <DialogDescription>Search TMDB and add a movie or series.</DialogDescription>
        <Input
          id="movies-search"
          type="search"
          aria-label="Search TMDB"
          placeholder="Movie or series name…"
          autoComplete="off"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        {searching && <MovieSearchResults query={debouncedQuery} />}
      </DialogContent>
    </Dialog>
  )
}

/** The backlog; adding a title is a header action. */
export default function MoviesClient() {
  const [filter, setFilter] = useState<BacklogFilter>(DEFAULT_BACKLOG_FILTER)
  const [adding, setAdding] = useState(false)
  const shared = useSharedList()
  const addButton = <Button onClick={() => setAdding(true)}>Add a title</Button>

  return (
    <PageContainer>
      <PageHeader
        title="Movies & series"
        actions={
          <>
            <PageHeaderLink href={listHref('/movies/stats', shared)}>Stats</PageHeaderLink>
            <PageHeaderSettingsLink href="/movies/settings" />
            {addButton}
          </>
        }
      />
      <ListSwitch className="mb-4" />
      <MoviesBacklog filter={filter} onFilterChange={setFilter} addAction={addButton} />
      <AddTitleDialog open={adding} onOpenChange={setAdding} />
      <TmdbAttribution />
    </PageContainer>
  )
}
