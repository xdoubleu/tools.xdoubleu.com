import React from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockSetStatus = vi.fn()

vi.mock('@/hooks/useMovies', () => ({
  useMoviesActions: vi.fn(() => ({ setStatus: mockSetStatus }))
}))

import { STATUS_LABELS, type Status } from '@/lib/movies/format'
import TitleStatus from '@/components/movies/TitleStatus'
import { BacklogEntrySchema, SeasonSchema } from '@/lib/gen/movies/v1/movies_pb'

const series = create(BacklogEntrySchema, {
  id: 's-1',
  mediaType: 'series',
  title: 'Dark',
  status: 'want'
})
const season = (number: number, aired: boolean) => create(SeasonSchema, { number, aired })

function choose(status: Status) {
  fireEvent.click(screen.getByRole('tab', { name: STATUS_LABELS[status] }))
}

beforeEach(() => {
  vi.clearAllMocks()
  mockSetStatus.mockResolvedValue(undefined)
})

describe('TitleStatus', () => {
  it('selects nothing for an unknown status', () => {
    const odd = create(BacklogEntrySchema, { id: 's-2', status: 'paused' })
    render(<TitleStatus entry={odd} seasons={[]} />)
    for (const tab of screen.getAllByRole('tab')) {
      expect(tab).toHaveAttribute('aria-selected', 'false')
    }
  })

  it('selects the entry status', () => {
    render(<TitleStatus entry={series} seasons={[]} />)
    expect(screen.getByRole('tab', { name: 'Want' })).toHaveAttribute('aria-selected', 'true')
  })

  it('marks a single-season series watched today in one step', async () => {
    // Specials and unaired seasons don't count.
    const seasons = [season(0, true), season(1, true), season(2, false)]
    render(<TitleStatus entry={series} seasons={seasons} />)
    await act(async () => choose('watched'))
    expect(mockSetStatus).toHaveBeenCalledWith('s-1', 'watched', false)
  })

  it.each([
    ['Today', false],
    ['A while ago', true]
  ])('asks when a multi-season series was watched: %s', async (choice, unknownDate) => {
    render(<TitleStatus entry={series} seasons={[season(1, true), season(2, true)]} />)
    choose('watched')
    expect(mockSetStatus).not.toHaveBeenCalled()

    await act(async () => fireEvent.click(screen.getByRole('button', { name: choice })))
    expect(mockSetStatus).toHaveBeenCalledWith('s-1', 'watched', unknownDate)
    expect(screen.queryByText('When did you watch Dark?')).not.toBeInTheDocument()
  })

  it('asks only for watched', async () => {
    render(<TitleStatus entry={series} seasons={[season(1, true), season(2, true)]} />)
    await act(async () => choose('dropped'))
    expect(mockSetStatus).toHaveBeenCalledWith('s-1', 'dropped', false)
  })

  it('shows saving progress and reports a failure until the next success', async () => {
    render(<TitleStatus entry={series} seasons={[season(1, true), season(2, true)]} />)
    expect(screen.getByRole('tab', { name: 'Watched' })).toBeEnabled()
    choose('watched')
    let resolve!: () => void
    mockSetStatus.mockReturnValueOnce(new Promise<void>((r) => (resolve = r)))
    fireEvent.click(screen.getByRole('button', { name: 'Today' }))
    expect(screen.getByRole('button', { name: 'Saving…' })).toBeDisabled()
    expect(screen.getByRole('tab', { name: 'Watched', hidden: true })).toBeDisabled()
    await act(async () => resolve())

    mockSetStatus.mockRejectedValueOnce(new Error('nope'))
    await act(async () => choose('dropped'))
    expect(screen.getByText(/Couldn.t update status/)).toBeInTheDocument()
    await act(async () => choose('want'))
    expect(screen.queryByText(/Couldn.t update status/)).not.toBeInTheDocument()
  })
})
