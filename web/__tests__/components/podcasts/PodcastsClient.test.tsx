import React from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'

jest.mock('@/components/podcasts/Episodes', () => ({
  __esModule: true,
  default: () => <div data-testid="episodes" />
}))
jest.mock('@/components/podcasts/Favourites', () => ({
  __esModule: true,
  default: () => <div data-testid="favourites" />
}))
jest.mock('@/components/podcasts/ShowSearchResults', () => ({
  __esModule: true,
  default: ({ query }: { query: string }) => <div data-testid="results">{query}</div>
}))

import PodcastsClient from '@/components/podcasts/PodcastsClient'

beforeEach(() => jest.useFakeTimers())
afterEach(() => jest.useRealTimers())

function type(value: string) {
  fireEvent.change(screen.getByLabelText('Search podcasts'), { target: { value } })
  act(() => {
    jest.advanceTimersByTime(300)
  })
}

describe('PodcastsClient', () => {
  it('switches between episodes and shows', () => {
    render(<PodcastsClient />)
    expect(screen.getByRole('tab', { name: 'Episodes' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.queryByTestId('favourites')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'Shows' }))
    expect(screen.getByTestId('favourites')).toBeInTheDocument()
    expect(screen.queryByTestId('episodes')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'Episodes' }))
    expect(screen.getByTestId('episodes')).toBeInTheDocument()
  })

  it('hides the tabs while searching and keeps the chosen view after', () => {
    render(<PodcastsClient />)
    fireEvent.click(screen.getByRole('tab', { name: 'Shows' }))
    type('hi')
    expect(screen.queryByRole('tab')).not.toBeInTheDocument()
    type('')
    expect(screen.getByTestId('favourites')).toBeInTheDocument()
  })

  it('shows episodes until a search of two characters', () => {
    render(<PodcastsClient />)
    expect(screen.getByTestId('episodes')).toBeInTheDocument()

    type('h')
    expect(screen.getByTestId('episodes')).toBeInTheDocument()

    type('hi')
    expect(screen.getByTestId('results')).toHaveTextContent('hi')
    expect(screen.queryByTestId('episodes')).not.toBeInTheDocument()

    type('')
    expect(screen.getByTestId('episodes')).toBeInTheDocument()
  })

  it('starts empty and waits for typing to pause before searching', () => {
    render(<PodcastsClient />)
    const box = screen.getByLabelText('Search podcasts')
    expect(box).toHaveValue('')

    fireEvent.change(box, { target: { value: 'hi' } })
    expect(screen.getByTestId('episodes')).toBeInTheDocument()
    act(() => {
      jest.advanceTimersByTime(299)
    })
    expect(screen.getByTestId('episodes')).toBeInTheDocument()
    act(() => {
      jest.advanceTimersByTime(1)
    })
    expect(screen.getByTestId('results')).toBeInTheDocument()
  })

  it('restarts the wait on every keystroke', () => {
    render(<PodcastsClient />)
    const box = screen.getByLabelText('Search podcasts')
    fireEvent.change(box, { target: { value: 'hi' } })
    act(() => {
      jest.advanceTimersByTime(200)
    })
    fireEvent.change(box, { target: { value: 'hij' } })
    act(() => {
      jest.advanceTimersByTime(200)
    })
    expect(screen.getByTestId('episodes')).toBeInTheDocument()
    act(() => {
      jest.advanceTimersByTime(100)
    })
    expect(screen.getByTestId('results')).toHaveTextContent('hij')
  })

  it('judges the live and the settled query by their trimmed length', () => {
    render(<PodcastsClient />)
    const box = screen.getByLabelText('Search podcasts')

    // A settled ' h' is one character; typing on to 'hi' must not show it.
    type(' h')
    fireEvent.change(box, { target: { value: 'hi' } })
    expect(screen.getByTestId('episodes')).toBeInTheDocument()

    // A settled 'hi' is hidden once the live box is down to ' h'.
    type('hi')
    expect(screen.getByTestId('results')).toBeInTheDocument()
    fireEvent.change(box, { target: { value: ' h' } })
    expect(screen.getByTestId('episodes')).toBeInTheDocument()
  })

  it('hides stale results as soon as the box is cleared', () => {
    render(<PodcastsClient />)
    type('hi')
    fireEvent.change(screen.getByLabelText('Search podcasts'), { target: { value: '' } })
    expect(screen.getByTestId('episodes')).toBeInTheDocument()
    expect(screen.queryByTestId('results')).not.toBeInTheDocument()
  })
})
