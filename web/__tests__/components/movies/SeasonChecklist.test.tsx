import React from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const actions = {
  setSeasonWatched: jest.fn(),
  addWatchDate: jest.fn(),
  editWatchDate: jest.fn(),
  removeWatchDate: jest.fn()
}

jest.mock('@/hooks/useMovies', () => ({
  useMoviesActions: jest.fn(() => actions)
}))

import SeasonChecklist from '@/components/movies/SeasonChecklist'
import { SeasonSchema } from '@/lib/gen/movies/v1/movies_pb'

const seasons = [
  create(SeasonSchema, { number: 0, name: 'Specials', episodeCount: 9, aired: false }),
  create(SeasonSchema, {
    number: 1,
    name: 'Season 1',
    airDate: '2008-01-20',
    episodeCount: 7,
    aired: true,
    watchedAt: ['']
  }),
  create(SeasonSchema, { number: 2, name: '', airDate: '2099-01-01', aired: false })
]

beforeEach(() => jest.clearAllMocks())

describe('SeasonChecklist', () => {
  it('renders nothing for no seasons', () => {
    const { container } = render(<SeasonChecklist entryId="e-1" seasons={[]} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('lists seasons with their state and details', () => {
    render(<SeasonChecklist entryId="e-1" seasons={seasons} />)
    expect(screen.getByLabelText('Specials')).not.toBeChecked()
    expect(screen.getByLabelText('Season 1')).toBeChecked()
    expect(screen.getByLabelText('Season 2')).not.toBeChecked()
    expect(
      screen.getByText("9 episodes · Not aired yet · doesn't count toward status")
    ).toBeInTheDocument()
    expect(screen.getByText('7 episodes · 2008-01-20')).toBeInTheDocument()
    expect(screen.getByText('Not aired yet')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Watch dates \(1\)/ })).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /Watch dates/ })).toHaveLength(1)
  })

  it('ticks and unticks seasons, locking the row while saving', async () => {
    let resolve!: () => void
    actions.setSeasonWatched.mockReturnValueOnce(new Promise<void>((r) => (resolve = r)))
    render(<SeasonChecklist entryId="e-1" seasons={seasons} />)

    fireEvent.click(screen.getByLabelText('Season 2'))
    expect(actions.setSeasonWatched).toHaveBeenCalledWith('e-1', 2, true)
    expect(screen.getByLabelText('Season 2')).toBeDisabled()
    await act(async () => resolve())
    expect(screen.getByLabelText('Season 2')).toBeEnabled()

    actions.setSeasonWatched.mockResolvedValueOnce(undefined)
    await act(async () => fireEvent.click(screen.getByLabelText('Season 1')))
    expect(actions.setSeasonWatched).toHaveBeenLastCalledWith('e-1', 1, false)
  })

  it('reports a failed save until the next one succeeds', async () => {
    render(<SeasonChecklist entryId="e-1" seasons={seasons} />)
    actions.setSeasonWatched.mockRejectedValueOnce(new Error('nope'))
    await act(async () => fireEvent.click(screen.getByLabelText('Season 2')))
    expect(screen.getByText(/Couldn.t save/)).toBeInTheDocument()

    actions.setSeasonWatched.mockResolvedValueOnce(undefined)
    await act(async () => fireEvent.click(screen.getByLabelText('Season 2')))
    expect(screen.queryByText(/Couldn.t save/)).not.toBeInTheDocument()
  })

  it("edits a ticked season's watch dates", async () => {
    actions.addWatchDate.mockResolvedValue(undefined)
    actions.editWatchDate.mockResolvedValue(undefined)
    actions.removeWatchDate.mockResolvedValue(undefined)
    render(<SeasonChecklist entryId="e-1" seasons={seasons} />)
    fireEvent.click(screen.getByRole('button', { name: /Watch dates/ }))

    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Add unknown date' })))
    expect(actions.addWatchDate).toHaveBeenCalledWith('e-1', 1, '')
    await act(async () =>
      fireEvent.change(screen.getByLabelText('Watch 1 date'), {
        target: { value: '2020-01-01' }
      })
    )
    expect(actions.editWatchDate).toHaveBeenCalledWith('e-1', 1, 0, '2020-01-01')
    expect(screen.getByLabelText('Watch 1 date')).toHaveAttribute('id', 'season-1-watch-0')
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Remove' })))
    expect(actions.removeWatchDate).toHaveBeenCalledWith('e-1', 1, 0)
  })
})
