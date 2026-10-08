'use client'

import { useEffect, useState } from 'react'
import Episodes from '@/components/podcasts/Episodes'
import Favourites from '@/components/podcasts/Favourites'
import ShowSearchResults from '@/components/podcasts/ShowSearchResults'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'

// Each search is an iTunes request; wait for typing to pause.
const SEARCH_DEBOUNCE_MS = 300

type View = 'episodes' | 'shows'

const VIEWS = [
  { value: 'episodes', label: 'Episodes' },
  { value: 'shows', label: 'Shows' }
] as const

/** New episodes by default; a two-character search swaps in iTunes
 * results. */
export default function PodcastsClient() {
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [view, setView] = useState<View>('episodes')
  // Clearing the box shows favourites at once; only the fetch waits.
  const searching = debouncedQuery.trim().length >= 2 && query.trim().length >= 2

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [query])

  return (
    <PageContainer>
      <PageHeader title="Podcasts" />
      <Field label="Search podcasts" htmlFor="podcasts-search" className="mb-6">
        <Input
          id="podcasts-search"
          type="search"
          placeholder="Name of a show"
          autoComplete="off"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </Field>
      {searching ? (
        <ShowSearchResults query={debouncedQuery} />
      ) : (
        <div className="space-y-4">
          <SegmentedTabs aria-label="View" value={view} options={[...VIEWS]} onChange={setView} />
          {view === 'episodes' ? <Episodes /> : <Favourites />}
        </div>
      )}
    </PageContainer>
  )
}
