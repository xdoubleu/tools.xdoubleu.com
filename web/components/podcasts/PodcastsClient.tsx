'use client'

import { useEffect, useState } from 'react'
import Favourites from '@/components/podcasts/Favourites'
import ShowSearchResults from '@/components/podcasts/ShowSearchResults'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

// Each search is an iTunes request; wait for typing to pause.
const SEARCH_DEBOUNCE_MS = 300

/** Favourites by default; a two-character search swaps in iTunes results. */
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
      {searching ? <ShowSearchResults query={debouncedQuery} /> : <Favourites />}
    </PageContainer>
  )
}
