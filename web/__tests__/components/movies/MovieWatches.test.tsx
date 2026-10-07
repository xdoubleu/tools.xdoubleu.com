import React from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const actions = {
  addWatchDate: jest.fn(),
  editWatchDate: jest.fn(),
  removeWatchDate: jest.fn()
}

jest.mock('@/hooks/useMovies', () => ({
  useMoviesActions: jest.fn(() => actions)
}))

import MovieWatches from '@/components/movies/MovieWatches'
import { BacklogEntrySchema } from '@/lib/gen/movies/v1/movies_pb'

const movie = create(BacklogEntrySchema, { id: 'm-1', mediaType: 'movie', watchedAt: [''] })

beforeEach(() => jest.clearAllMocks())

describe('MovieWatches', () => {
  it("edits the movie's own watches", async () => {
    actions.addWatchDate.mockResolvedValue(undefined)
    actions.editWatchDate.mockResolvedValue(undefined)
    actions.removeWatchDate.mockResolvedValue(undefined)
    render(<MovieWatches entry={movie} />)

    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Add unknown date' })))
    expect(actions.addWatchDate).toHaveBeenCalledWith('m-1', undefined, '')
    await act(async () =>
      fireEvent.change(screen.getByLabelText('Watch 1 date'), {
        target: { value: '2021-05-06' }
      })
    )
    expect(actions.editWatchDate).toHaveBeenCalledWith('m-1', undefined, 0, '2021-05-06')
    expect(screen.getByLabelText('Watch 1 date')).toHaveAttribute('id', 'movie-watch-0')
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Remove' })))
    expect(actions.removeWatchDate).toHaveBeenCalledWith('m-1', undefined, 0)
  })

  it('locks while saving and reports a failure until the next success', async () => {
    render(<MovieWatches entry={movie} />)
    expect(screen.getByRole('button', { name: 'Remove' })).toBeEnabled()

    let resolve!: () => void
    actions.removeWatchDate.mockReturnValueOnce(new Promise<void>((r) => (resolve = r)))
    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))
    expect(screen.getByRole('button', { name: 'Remove' })).toBeDisabled()
    await act(async () => resolve())
    expect(screen.getByRole('button', { name: 'Remove' })).toBeEnabled()

    actions.addWatchDate.mockRejectedValueOnce(new Error('nope'))
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Watched today' })))
    expect(screen.getByText(/Couldn.t save/)).toBeInTheDocument()
    actions.addWatchDate.mockResolvedValueOnce(undefined)
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Watched today' })))
    expect(screen.queryByText(/Couldn.t save/)).not.toBeInTheDocument()
  })
})
