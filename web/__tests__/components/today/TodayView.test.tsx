import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import TodayView from '@/components/today/TodayView'
import { GetTodayResponseSchema } from '@/lib/gen/dashboard/v1/today_pb'

vi.mock('@/hooks/useToday', () => ({ useToday: vi.fn() }))

import { useToday } from '@/hooks/useToday'

const mockUseToday = vi.mocked(useToday)

function show(value: Parameters<typeof create<typeof GetTodayResponseSchema>>[1]) {
  // @ts-expect-error -- partial SWR response for the test
  mockUseToday.mockReturnValue({ data: create(GetTodayResponseSchema, value), error: undefined })
  render(<TodayView />)
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('TodayView', () => {
  it('shows skeletons while loading', () => {
    // @ts-expect-error -- partial SWR response for the test
    mockUseToday.mockReturnValue({ data: undefined, error: undefined })
    const { container } = render(<TodayView />)
    expect(container.querySelectorAll('[aria-busy="true"] > div')).toHaveLength(4)
  })

  it('shows an error when nothing has loaded', () => {
    // @ts-expect-error -- partial SWR response for the test
    mockUseToday.mockReturnValue({ data: undefined, error: new Error('boom') })
    render(<TodayView />)
    expect(screen.getByRole('alert')).toHaveTextContent('Failed to load your day.')
  })

  it('keeps showing the last summary when a refetch fails', () => {
    // @ts-expect-error -- partial SWR response for the test
    mockUseToday.mockReturnValue({
      data: create(GetTodayResponseSchema, { shopping: { itemCount: 2 } }),
      error: new Error('boom')
    })
    render(<TodayView />)
    expect(screen.getByText('2 items on your list')).toBeInTheDocument()
  })

  it('welcomes a user with nothing to show', () => {
    show({})
    expect(screen.getByText(/Nothing needs you today/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Browse apps' })).toHaveAttribute('href', '/apps')
  })

  it('treats empty blocks as nothing to show', () => {
    show({ reading: { books: [] }, feeds: { unreadCount: 0, items: [] }, shopping: {} })
    expect(screen.getByText(/Nothing needs you today/)).toBeInTheDocument()
  })

  it('renders every block with its links', () => {
    show({
      factory: { tasks: [{ ref: 'T-5', title: 'Fix it', status: 'needs_you' }] },
      meals: {
        meals: [
          { date: '2026-11-12', slot: 'evening', name: 'Soup' },
          { date: '2026-11-12', slot: 'late', name: 'Snack' }
        ]
      },
      reading: { books: [{ id: 'b1', title: 'Dune', authors: [], progressPercent: 40 }] },
      feeds: {
        unreadCount: 7,
        items: [{ id: 'i1', title: 'Post', feedTitle: 'Blog', url: 'https://example.com/p' }]
      },
      watching: {
        titles: [
          { id: 'm1', title: 'Dark', hasNewSeason: true },
          { id: 'm2', title: 'Alien', hasNewSeason: false }
        ]
      },
      learning: { paths: [{ id: 'p1', title: 'Rust' }] },
      shopping: { itemCount: 1 }
    })

    expect(screen.getByRole('link', { name: /T-5 Fix it/ })).toHaveAttribute(
      'href',
      '/factory?task=T-5'
    )
    expect(screen.getByText('Needs you', { selector: 'span' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Soup/ })).toHaveAttribute('href', '/mealplans')
    expect(screen.getByText('Dinner')).toBeInTheDocument()
    expect(screen.getByText('late')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Dune' })).toHaveAttribute('href', '/books/b1')
    expect(screen.getByRole('progressbar', { name: 'Dune progress' })).toHaveAttribute(
      'aria-valuetext',
      '40%'
    )
    expect(screen.getByRole('link', { name: '7 unread' })).toHaveAttribute('href', '/feeds')
    expect(screen.getByRole('link', { name: /Post/ })).toHaveAttribute(
      'href',
      'https://example.com/p'
    )
    expect(screen.getByRole('link', { name: /Dark/ })).toHaveAttribute('href', '/movies/m1')
    expect(screen.getByText('New season')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Alien/ })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Rust' })).toHaveAttribute('href', '/learningpaths/p1')
    expect(screen.getByText('1 item on your list')).toBeInTheDocument()
  })

  it('labels a task in testing', () => {
    show({ factory: { tasks: [{ ref: 'T-6', title: 'Check', status: 'testing' }] } })
    expect(screen.getByText('Testing')).toBeInTheDocument()
  })

  const single = {
    'Needs you': { factory: { tasks: [{ ref: 'T-1', title: 'a', status: 'testing' }] } },
    Meals: { meals: { meals: [{ date: 'd', slot: 'noon', name: 'a' }] } },
    Reading: { reading: { books: [{ id: 'b', title: 'a', authors: [], progressPercent: 1 }] } },
    Feeds: { feeds: { unreadCount: 1, items: [] } },
    Watching: { watching: { titles: [{ id: 'm', title: 'a', hasNewSeason: false }] } },
    Learning: { learning: { paths: [{ id: 'p', title: 'a' }] } },
    'Shopping list': { shopping: { itemCount: 1 } }
  }
  const emptyBlocks = {
    factory: { tasks: [] },
    meals: { meals: [] },
    reading: { books: [] },
    feeds: { unreadCount: 0, items: [] },
    watching: { titles: [] },
    learning: { paths: [] },
    shopping: { itemCount: 0 }
  }

  it.each(Object.entries(single))(
    'shows only the %s block when the others are empty',
    (title, v) => {
      show({ ...emptyBlocks, ...v })
      expect(screen.getByText(title)).toBeInTheDocument()
      expect(screen.queryByText(/Nothing needs you today/)).not.toBeInTheDocument()
      for (const other of Object.keys(single).filter((t) => t !== title)) {
        expect(screen.queryByText(other)).not.toBeInTheDocument()
      }
    }
  )

  it.each([
    ['breakfast', 'Breakfast'],
    ['noon', 'Lunch'],
    ['evening', 'Dinner']
  ])('labels the %s slot', (slot, label) => {
    show({ meals: { meals: [{ date: 'd', slot, name: 'x' }] } })
    expect(screen.getByText(label)).toBeInTheDocument()
  })

  it('renders no detail for a row without one', () => {
    show({ learning: { paths: [{ id: 'p', title: 'Rust' }] } })
    expect(screen.getByRole('link', { name: 'Rust' }).children).toHaveLength(1)
  })
})
