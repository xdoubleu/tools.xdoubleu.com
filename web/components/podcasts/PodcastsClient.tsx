import { useEffect, useState } from 'react'
import Favourites from '@/components/podcasts/Favourites'
import ShowSearchResults from '@/components/podcasts/ShowSearchResults'
import { Input } from '@/components/ui/input'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { Toolbar } from '@/components/ui/toolbar'

// Each search is an iTunes request; wait for typing to pause.
const SEARCH_DEBOUNCE_MS = 300

/** The favourites list; a two-character search swaps in iTunes results. */
export default function PodcastsClient() {
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  // Clearing the box shows favourites at once; only the fetch waits.
  const searching = debouncedQuery.trim().length >= 2 && query.trim().length >= 2

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [query])

  return (
    <PageContainer>
      <PageHeader title="Podcasts" />
      <Toolbar
        aria-label="Find a show"
        search={
          <Input
            id="podcasts-search"
            type="search"
            aria-label="Search podcasts"
            placeholder="Search podcasts by show name…"
            autoComplete="off"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        }
      />
      {searching ? <ShowSearchResults query={debouncedQuery} /> : <Favourites />}
    </PageContainer>
  )
}
