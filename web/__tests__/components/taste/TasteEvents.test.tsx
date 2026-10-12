import React from 'react'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockFetchPage = jest.fn()

jest.mock('@/hooks/useTaste', () => ({
  useTasteEvents: jest.fn(),
  useFetchTasteEventsPage: jest.fn(() => mockFetchPage)
}))

import TasteEvents from '@/components/taste/TasteEvents'
import { useFetchTasteEventsPage, useTasteEvents } from '@/hooks/useTaste'
import { type Event, EventSchema, ListEventsResponseSchema } from '@/lib/gen/taste/v1/taste_pb'

const dune = create(EventSchema, {
  domain: 'books',
  itemId: 'b1',
  title: 'Dune',
  creator: 'Frank Herbert',
  at: '2026-03-04T05:06:07Z',
  signal: 2,
  reason: 'rated 5★'
})
const bounced = create(EventSchema, {
  domain: 'games',
  itemId: 'g1',
  title: 'Outer Wilds',
  signal: -1
})

function mockEvents(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  jest.mocked(useTasteEvents).mockReturnValue({ isLoading: false, ...value })
}

function page(events: Event[], hasMore = false) {
  return create(ListEventsResponseSchema, { events, hasMore })
}

beforeEach(() => jest.clearAllMocks())

describe('TasteEvents', () => {
  it('shows each event with its domain, creator, date, reason and signal', () => {
    mockEvents({ data: page([dune, bounced]) })
    render(<TasteEvents />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.getByText('Books · Frank Herbert · 04/03/2026')).toBeInTheDocument()
    expect(screen.getByText('rated 5★')).toBeInTheDocument()
    expect(screen.getByText('Loved')).toBeInTheDocument()
    const log = screen.getByRole('list', { name: 'Consumption log' })
    expect(within(log).getByText('Games')).toBeInTheDocument()
    // No reason, no reason line.
    const bouncedRow = within(log).getAllByRole('listitem')[1]
    expect(bouncedRow.querySelectorAll('p')).toHaveLength(2)
    expect(screen.getByText('Disliked')).toBeInTheDocument()
  })

  it('shows loading, error and empty states', () => {
    mockEvents({ isLoading: true })
    const { rerender } = render(<TasteEvents />)
    expect(screen.getByText(/Loading log/)).toBeInTheDocument()

    // Not loading and no data yet: nothing to show, not a spinner.
    mockEvents({})
    rerender(<TasteEvents />)
    expect(screen.queryByText(/Loading log/)).not.toBeInTheDocument()

    mockEvents({ error: new Error('nope') })
    rerender(<TasteEvents />)
    expect(screen.getByRole('alert')).toBeInTheDocument()

    mockEvents({ data: page([]) })
    rerender(<TasteEvents />)
    expect(screen.getByText('Nothing here yet.')).toBeInTheDocument()
  })

  it('filters by domain', () => {
    mockEvents({ data: page([dune]) })
    render(<TasteEvents />)
    expect(useTasteEvents).toHaveBeenLastCalledWith('')
    fireEvent.change(screen.getByLabelText('Domain'), { target: { value: 'games' } })
    expect(useTasteEvents).toHaveBeenLastCalledWith('games')
    expect(useFetchTasteEventsPage).toHaveBeenLastCalledWith('games')
  })

  it('loads more pages', async () => {
    mockEvents({ data: page([dune], true) })
    mockFetchPage.mockResolvedValue({ items: [bounced], hasMore: false })
    render(<TasteEvents />)
    fireEvent.click(screen.getByRole('button', { name: /Load more/ }))
    await waitFor(() => expect(screen.getByText('Outer Wilds')).toBeInTheDocument())
    expect(mockFetchPage).toHaveBeenCalledWith(1)
    expect(screen.queryByRole('button', { name: /Load more/ })).not.toBeInTheDocument()
  })
})

describe('TasteEvents revalidation', () => {
  const extra = create(EventSchema, { domain: 'movies', itemId: 'm1', title: 'Arrival' })

  async function loadTwoPages() {
    mockEvents({ data: page([dune], true) })
    mockFetchPage.mockResolvedValue({ items: [bounced], hasMore: false })
    const view = render(<TasteEvents />)
    fireEvent.click(screen.getByRole('button', { name: /Load more/ }))
    await waitFor(() => expect(screen.getByText('Outer Wilds')).toBeInTheDocument())
    return view
  }

  it('keeps loaded pages when the first page comes back unchanged', async () => {
    const { rerender } = await loadTwoPages()
    mockEvents({ data: page([create(EventSchema, { ...dune })], true) })
    rerender(<TasteEvents />)
    expect(screen.getByText('Outer Wilds')).toBeInTheDocument()
  })

  it('resets when the first page changes item', async () => {
    const { rerender } = await loadTwoPages()
    mockEvents({ data: page([extra], true) })
    rerender(<TasteEvents />)
    await waitFor(() => expect(screen.getByText('Arrival')).toBeInTheDocument())
    expect(screen.queryByText('Outer Wilds')).not.toBeInTheDocument()
    expect(screen.queryByText('Dune')).not.toBeInTheDocument()
  })

  it.each([
    ['another domain with the same id', { domain: 'games', itemId: 'b1' }],
    ['the same domain with another id', { domain: 'books', itemId: 'b2' }]
  ])('treats %s as a different item', async (_, key) => {
    const { rerender } = await loadTwoPages()
    mockEvents({ data: page([create(EventSchema, { ...dune, ...key, title: 'Dune 2' })], true) })
    rerender(<TasteEvents />)
    await waitFor(() => expect(screen.getByText('Dune 2')).toBeInTheDocument())
    expect(screen.queryByText('Outer Wilds')).not.toBeInTheDocument()
  })

  it('keys rows uniquely', () => {
    const spy = jest.spyOn(console, 'error').mockImplementation(() => {})
    mockEvents({ data: page([dune, bounced]) })
    render(<TasteEvents />)
    expect(spy).not.toHaveBeenCalled()
    spy.mockRestore()
  })
})
