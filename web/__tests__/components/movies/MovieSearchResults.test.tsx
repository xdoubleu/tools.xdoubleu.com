import React from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockAdd = jest.fn()

jest.mock('@/hooks/useMovies', () => ({
  useMovieSearch: jest.fn(),
  useMoviesActions: jest.fn(() => ({ add: mockAdd }))
}))

import MovieSearchResults from '@/components/movies/MovieSearchResults'
import { useMovieSearch } from '@/hooks/useMovies'
import { SearchResultSchema, SearchTitlesResponseSchema } from '@/lib/gen/movies/v1/movies_pb'

const results = create(SearchTitlesResponseSchema, {
  results: [
    create(SearchResultSchema, {
      mediaType: 'movie',
      tmdbId: 438631n,
      title: 'Dune',
      releaseDate: '2021-09-15'
    }),
    create(SearchResultSchema, { mediaType: 'series', tmdbId: 1n, title: 'Untitled' })
  ]
})

function mockSearch(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  jest.mocked(useMovieSearch).mockReturnValue({ isLoading: false, ...value })
}

function deferred() {
  let resolve!: () => void
  const promise = new Promise<void>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

beforeEach(() => jest.clearAllMocks())

describe('MovieSearchResults', () => {
  it('shows type and release, or type alone when undated', () => {
    mockSearch({ data: results })
    render(<MovieSearchResults query="dune" />)
    const metas = screen.getAllByText(/^(Movie|Series)/, { selector: 'p' })
    expect(metas.map((p) => p.textContent)).toEqual(['Movie · 2021', 'Series'])
    expect(screen.queryByText(/Couldn.t add/)).not.toBeInTheDocument()
  })

  it.each([
    ['Want', 'want'],
    ['Watched', 'watched']
  ])('marks %s as adding and locks both buttons until done', async (label, status) => {
    mockSearch({ data: results })
    render(<MovieSearchResults query="dune" />)
    const [row] = screen.getAllByRole('listitem')
    const buttons = () => row.querySelectorAll('button')

    const add = deferred()
    mockAdd.mockReturnValueOnce(add.promise)
    fireEvent.click(screen.getAllByRole('button', { name: label })[0])
    expect(mockAdd).toHaveBeenCalledWith('movie', 438631n, status)
    expect(row).toHaveTextContent('Adding…')
    buttons().forEach((b) => expect(b).toBeDisabled())

    await act(async () => add.resolve())
    expect(row).not.toHaveTextContent('Adding…')
    buttons().forEach((b) => expect(b).toBeEnabled())
  })

  it('clears a failed add on retry', async () => {
    mockSearch({ data: results })
    render(<MovieSearchResults query="dune" />)
    const want = () => screen.getAllByRole('button', { name: 'Want' })[0]

    mockAdd.mockRejectedValueOnce(new Error('nope'))
    await act(async () => fireEvent.click(want()))
    expect(screen.getByText(/Couldn.t add/)).toBeInTheDocument()

    const add = deferred()
    mockAdd.mockReturnValueOnce(add.promise)
    fireEvent.click(want())
    expect(screen.queryByText(/Couldn.t add/)).not.toBeInTheDocument()
    await act(async () => add.resolve())
  })

  it('keeps a failed add on its own row when results change', async () => {
    mockSearch({ data: results })
    const { rerender } = render(<MovieSearchResults query="dune" />)
    mockAdd.mockRejectedValueOnce(new Error('nope'))
    await act(async () => fireEvent.click(screen.getAllByRole('button', { name: 'Want' })[1]))

    mockSearch({ data: create(SearchTitlesResponseSchema, { results: [results.results[1]] }) })
    rerender(<MovieSearchResults query="dune" />)
    expect(screen.getByRole('listitem')).toHaveTextContent(/Couldn.t add/)
  })

  it('keeps previous results while a new search loads', () => {
    mockSearch({ data: results, isLoading: true })
    render(<MovieSearchResults query="dune" />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.queryByText('Loading results…')).not.toBeInTheDocument()
  })

  it('quotes the trimmed query when nothing matches', () => {
    mockSearch({ data: create(SearchTitlesResponseSchema, { results: [] }) })
    render(<MovieSearchResults query="  zz " />)
    expect(screen.getByText('No movies or series match “zz”.')).toBeInTheDocument()
  })
})
