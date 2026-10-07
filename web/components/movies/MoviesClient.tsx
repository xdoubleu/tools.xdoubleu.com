'use client'

import { useEffect, useState } from 'react'
import MovieSearchResults from '@/components/movies/MovieSearchResults'
import MoviesBacklog from '@/components/movies/MoviesBacklog'
import TmdbAttribution from '@/components/movies/TmdbAttribution'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderLink } from '@/components/ui/page-header'

// Each search is a TMDB request; wait for typing to pause.
const SEARCH_DEBOUNCE_MS = 300

/** Backlog by default; a two-character search swaps in TMDB results. */
export default function MoviesClient() {
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const searching = debouncedQuery.trim().length >= 2

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [query])

  return (
    <PageContainer>
      <PageHeader
        title="Movies & Series"
        actions={<PageHeaderLink href="/movies/stats">Stats</PageHeaderLink>}
      />
      <Field label="Search TMDB" htmlFor="movies-search" className="mb-6">
        <Input
          id="movies-search"
          type="search"
          placeholder="Title of a movie or series"
          autoComplete="off"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </Field>
      {searching ? <MovieSearchResults query={debouncedQuery} /> : <MoviesBacklog />}
      <TmdbAttribution />
    </PageContainer>
  )
}
