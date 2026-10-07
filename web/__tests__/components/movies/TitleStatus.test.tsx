import React from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockSetStatus = jest.fn()

jest.mock('@/hooks/useMovies', () => ({
  useMoviesActions: jest.fn(() => ({ setStatus: mockSetStatus }))
}))

import TitleStatus from '@/components/movies/TitleStatus'
import { BacklogEntrySchema, SeasonSchema } from '@/lib/gen/movies/v1/movies_pb'

const series = create(BacklogEntrySchema, {
  id: 's-1',
  mediaType: 'series',
  title: 'Dark',
  status: 'want'
})
const season = (number: number, aired: boolean) => create(SeasonSchema, { number, aired })

function choose(status: string) {
  fireEvent.change(screen.getByLabelText('Status'), { target: { value: status } })
}

beforeEach(() => {
  jest.clearAllMocks()
  mockSetStatus.mockResolvedValue(undefined)
})

describe('TitleStatus', () => {
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
    expect(screen.getByLabelText('Status')).toBeEnabled()
    choose('watched')
    let resolve!: () => void
    mockSetStatus.mockReturnValueOnce(new Promise<void>((r) => (resolve = r)))
    fireEvent.click(screen.getByRole('button', { name: 'Today' }))
    expect(screen.getByRole('button', { name: 'Saving…' })).toBeDisabled()
    expect(screen.getByLabelText('Status')).toBeDisabled()
    await act(async () => resolve())

    mockSetStatus.mockRejectedValueOnce(new Error('nope'))
    await act(async () => choose('dropped'))
    expect(screen.getByText(/Couldn.t update status/)).toBeInTheDocument()
    await act(async () => choose('want'))
    expect(screen.queryByText(/Couldn.t update status/)).not.toBeInTheDocument()
  })
})
