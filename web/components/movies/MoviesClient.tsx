'use client'

import { useState } from 'react'
import MovieSearchResults from '@/components/movies/MovieSearchResults'
import MoviesBacklog from '@/components/movies/MoviesBacklog'
import TmdbAttribution from '@/components/movies/TmdbAttribution'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

/** Backlog by default; a two-character search swaps in TMDB results. */
export default function MoviesClient() {
  const [query, setQuery] = useState('')
  const searching = query.trim().length >= 2

  return (
    <PageContainer>
      <PageHeader title="Movies & Series" />
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
      {searching ? <MovieSearchResults query={query} /> : <MoviesBacklog />}
      <TmdbAttribution />
    </PageContainer>
  )
}
