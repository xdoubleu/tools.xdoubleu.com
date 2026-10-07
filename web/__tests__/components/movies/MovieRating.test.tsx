import React from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockSetRating = jest.fn()

jest.mock('@/hooks/useMovies', () => ({
  useMoviesActions: jest.fn(() => ({ setRating: mockSetRating }))
}))

import MovieRating from '@/components/movies/MovieRating'
import { BacklogEntrySchema } from '@/lib/gen/movies/v1/movies_pb'

const entry = (status: string, rating?: number) =>
  create(BacklogEntrySchema, { id: 'm-1', title: 'Dune', status, rating })

beforeEach(() => jest.clearAllMocks())

describe('MovieRating', () => {
  it('stays hidden until watched or rated', () => {
    const { rerender } = render(<MovieRating entry={entry('want')} />)
    expect(screen.queryByRole('region', { name: 'Rating' })).not.toBeInTheDocument()

    rerender(<MovieRating entry={entry('watched')} />)
    expect(screen.getByLabelText('No rating')).toBeInTheDocument()

    rerender(<MovieRating entry={entry('watching', 4)} />)
    expect(screen.getByLabelText('4 out of 5 stars')).toBeInTheDocument()
  })

  it('shows the clicked rating while saving, then the saved one', async () => {
    let resolve!: () => void
    mockSetRating.mockReturnValueOnce(new Promise<void>((r) => (resolve = r)))
    const { rerender } = render(<MovieRating entry={entry('watched')} />)

    fireEvent.click(screen.getByRole('button', { name: 'Rate 4 stars' }))
    expect(mockSetRating).toHaveBeenCalledWith('m-1', 4)
    expect(screen.getByLabelText('4 out of 5 stars')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Rate 4 stars' })).toBeDisabled()

    rerender(<MovieRating entry={entry('watched', 4)} />)
    await act(async () => resolve())
    expect(screen.getByRole('button', { name: 'Rate 4 stars' })).toBeEnabled()
    expect(screen.getByLabelText('4 out of 5 stars')).toBeInTheDocument()
  })

  it('clears by clicking the current rating', async () => {
    mockSetRating.mockResolvedValueOnce(undefined)
    render(<MovieRating entry={entry('watched', 3)} />)
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Rate 3 stars' })))
    expect(mockSetRating).toHaveBeenCalledWith('m-1', 0)
  })

  it('reverts and reports a failed save until the next one', async () => {
    mockSetRating.mockRejectedValueOnce(new Error('nope'))
    render(<MovieRating entry={entry('watched', 2)} />)
    expect(screen.queryByText(/Couldn.t save/)).not.toBeInTheDocument()

    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Rate 5 stars' })))
    expect(screen.getByText(/Couldn.t save/)).toBeInTheDocument()
    expect(screen.getByLabelText('2 out of 5 stars')).toBeInTheDocument()

    mockSetRating.mockResolvedValueOnce(undefined)
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Rate 5 stars' })))
    expect(screen.queryByText(/Couldn.t save/)).not.toBeInTheDocument()
  })
})
