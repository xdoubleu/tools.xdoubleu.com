import React from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'

vi.mock('@/components/podcasts/Favourites', () => ({
  __esModule: true,
  default: ({ addAction }: { addAction?: React.ReactNode }) => (
    <div data-testid="favourites">{addAction}</div>
  )
}))
vi.mock('@/components/podcasts/ShowSearchResults', () => ({
  __esModule: true,
  default: ({ query }: { query: string }) => <div data-testid="results">{query}</div>
}))

import PodcastsClient from '@/components/podcasts/PodcastsClient'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

function openDialog() {
  fireEvent.click(screen.getAllByRole('button', { name: 'Add a show' })[0])
}

function type(value: string) {
  fireEvent.change(screen.getByLabelText('Search iTunes'), { target: { value } })
  act(() => {
    vi.advanceTimersByTime(300)
  })
}

describe('PodcastsClient', () => {
  it('lists favourites, with adding a header action that opens the iTunes search', () => {
    render(<PodcastsClient />)
    expect(screen.getByTestId('favourites')).toBeInTheDocument()
    expect(screen.queryByLabelText('Search iTunes')).not.toBeInTheDocument()

    openDialog()
    expect(screen.getByRole('dialog', { name: 'Add a show' })).toBeInTheDocument()
    expect(screen.getByLabelText('Search iTunes')).toHaveValue('')
  })

  it('searches only from two characters', () => {
    render(<PodcastsClient />)
    openDialog()
    type('h')
    expect(screen.queryByTestId('results')).not.toBeInTheDocument()

    type('hi')
    expect(screen.getByTestId('results')).toHaveTextContent('hi')
  })

  it('waits for typing to pause before searching', () => {
    render(<PodcastsClient />)
    openDialog()
    fireEvent.change(screen.getByLabelText('Search iTunes'), { target: { value: 'hi' } })
    act(() => {
      vi.advanceTimersByTime(299)
    })
    expect(screen.queryByTestId('results')).not.toBeInTheDocument()
    act(() => {
      vi.advanceTimersByTime(1)
    })
    expect(screen.getByTestId('results')).toBeInTheDocument()
  })

  it('restarts the wait on every keystroke', () => {
    render(<PodcastsClient />)
    openDialog()
    const box = screen.getByLabelText('Search iTunes')
    fireEvent.change(box, { target: { value: 'hi' } })
    act(() => {
      vi.advanceTimersByTime(200)
    })
    fireEvent.change(box, { target: { value: 'hij' } })
    act(() => {
      vi.advanceTimersByTime(200)
    })
    expect(screen.queryByTestId('results')).not.toBeInTheDocument()
    act(() => {
      vi.advanceTimersByTime(100)
    })
    expect(screen.getByTestId('results')).toHaveTextContent('hij')
  })

  it('judges the live and the settled query by their trimmed length', () => {
    render(<PodcastsClient />)
    openDialog()
    const box = screen.getByLabelText('Search iTunes')

    // A settled ' h' is one character; typing on to 'hi' must not show it.
    type(' h')
    fireEvent.change(box, { target: { value: 'hi' } })
    expect(screen.queryByTestId('results')).not.toBeInTheDocument()

    // A settled 'hi' is hidden once the live box is down to ' h'.
    type('hi')
    expect(screen.getByTestId('results')).toBeInTheDocument()
    fireEvent.change(box, { target: { value: ' h' } })
    expect(screen.queryByTestId('results')).not.toBeInTheDocument()
  })

  it('hides stale results as soon as the box is cleared', () => {
    render(<PodcastsClient />)
    openDialog()
    type('hi')
    fireEvent.change(screen.getByLabelText('Search iTunes'), { target: { value: '' } })
    expect(screen.queryByTestId('results')).not.toBeInTheDocument()
  })
})
