import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema, LibraryResponseSchema } from '@/lib/gen/books/v1/library_pb'
import type { DashboardChartState } from '@/hooks/useDashboardChartState'

jest.mock('@/components/books/BooksProgressChart', () => () => (
  <div data-testid="books-progress-chart" />
))

import BooksDashboardView from '@/components/books/BooksDashboardView'

function makeLibrary() {
  return create(LibraryResponseSchema, {
    reading: [
      create(UserBookSchema, {
        id: 'ub-1',
        status: 'currently-reading',
        book: create(BookSchema, { title: 'Reading Book', authors: ['Author A'] })
      })
    ],
    wishlist: [create(UserBookSchema, { id: 'ub-2', status: 'to-read' })],
    finished: [create(UserBookSchema, { id: 'ub-3', status: 'read' })]
  })
}

function makeChart(view: 'ytd' | 'all' = 'ytd'): DashboardChartState<'ytd' | 'all'> {
  return {
    view,
    setView: jest.fn(),
    start: '2025-01-01',
    setStart: jest.fn(),
    end: '2026-01-01',
    setEnd: jest.fn()
  }
}

function renderView(overrides: Partial<Parameters<typeof BooksDashboardView>[0]> = {}) {
  return render(
    <BooksDashboardView
      library={makeLibrary()}
      chart={makeChart()}
      allTimeChartData={[]}
      renderReadingCard={(ub) => <span>card-{ub.id}</span>}
      actions={null}
      {...overrides}
    />
  )
}

describe('BooksDashboardView', () => {
  it('renders stat cards derived from the library', () => {
    renderView()
    expect(screen.getByText('Total books')).toBeInTheDocument()
    expect(screen.getByText('Read this year')).toBeInTheDocument()
  })

  it('shows per-series progress only when the library has series', () => {
    const { unmount } = renderView()
    expect(screen.queryByText('Series')).not.toBeInTheDocument()
    unmount()
    const library = create(LibraryResponseSchema, {
      finished: [
        create(UserBookSchema, {
          id: 'ub-9',
          status: 'read',
          book: create(BookSchema, { title: 'Mort', seriesName: 'Discworld', seriesTotal: 4 })
        })
      ]
    })
    renderView({ library })
    expect(screen.getByText('Series')).toBeInTheDocument()
    expect(screen.getByText('1 of 4 read')).toBeInTheDocument()
    const bar = screen.getByRole('progressbar', { name: 'Discworld progress' })
    expect(bar).toHaveAttribute('aria-valuenow', '25')
    expect(bar.firstElementChild).toHaveStyle({ width: '25%' })
  })

  it('renders the supplied reading card', () => {
    renderView()
    expect(screen.getByText('card-ub-1')).toBeInTheDocument()
  })

  it('requests a view change when the All time tab is clicked', () => {
    const chart = makeChart('ytd')
    renderView({ chart })
    expect(screen.queryByLabelText('From')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab', { name: 'All time' }))
    expect(chart.setView).toHaveBeenCalledWith('all')
  })

  it('shows the all-time chart and date inputs in the all view', () => {
    renderView({ chart: makeChart('all'), allTimeChartData: [{ label: 'Jan', value: 1 }] })
    expect(screen.getByTestId('books-progress-chart')).toBeInTheDocument()
    expect(screen.getByLabelText('From')).toBeInTheDocument()
    expect(screen.getByLabelText('To')).toBeInTheDocument()
  })
})
