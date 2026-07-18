import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  LibraryResponseSchema,
  UserBookSchema,
  BookSchema,
  BookShelfSchema
} from '@/lib/gen/reading/v1/library_pb'
import { GetSharedLibraryResponseSchema } from '@/lib/gen/reading/v1/public_pb'

const mockUseSharedLibrary = jest.fn()
const mockUseSharedBooksProgress = jest.fn()

jest.mock('@/hooks/useProfile', () => ({
  useSharedLibrary: () => mockUseSharedLibrary(),
  useSharedBooksProgress: () => mockUseSharedBooksProgress()
}))

jest.mock('@/components/reading/BooksProgressChart', () => () => (
  <div data-testid="books-progress-chart" />
))

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

import ProfileBooksClient from '@/components/profile/ProfileBooksClient'

function makeLibrary() {
  return create(GetSharedLibraryResponseSchema, {
    library: create(LibraryResponseSchema, {
      reading: [
        create(UserBookSchema, {
          id: 'ub-1',
          status: 'currently-reading',
          tags: [],
          progressPercent: 40,
          book: create(BookSchema, { title: 'Reading Book', authors: ['Author A'] })
        })
      ],
      wishlist: [
        create(UserBookSchema, {
          id: 'ub-2',
          status: 'to-read',
          tags: ['favourite', 'sci-fi'],
          rating: 4,
          book: create(BookSchema, { title: 'Wishlist Book', authors: ['Author B'] })
        })
      ],
      finished: [],
      shelves: [
        create(BookShelfSchema, {
          name: 'custom-shelf',
          books: [
            create(UserBookSchema, {
              id: 'ub-3',
              status: 'custom-shelf',
              tags: [],
              book: create(BookSchema, { title: 'Shelved Book', authors: ['Author C'] })
            })
          ]
        })
      ],
      rss: [
        create(UserBookSchema, {
          id: 'ub-rss',
          status: 'read',
          tags: [],
          book: create(BookSchema, { title: 'RSS Post', authors: ['Author D'], category: 'rss' })
        })
      ]
    }),
    lastSyncedAt: '2026-07-01T10:00:00Z'
  })
}

describe('ProfileBooksClient', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUseSharedBooksProgress.mockReturnValue({ data: undefined })
  })

  it('renders separate RSS stat cards', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksClient token="tok-1" />)
    expect(screen.getByText('RSS items')).toBeInTheDocument()
    expect(screen.getByText('RSS read')).toBeInTheDocument()
  })

  it('renders stat cards and last synced state', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksClient token="tok-1" />)

    expect(screen.getByText('Total books')).toBeInTheDocument()
    expect(screen.getByText('Read this year')).toBeInTheDocument()
    expect(screen.getByText(/Last synced:/)).toBeInTheDocument()
  })

  it('links to the shared library and omits the inline library', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksClient token="tok-1" />)

    const link = screen.getByRole('link', { name: 'Browse full library' })
    expect(link).toHaveAttribute('href', '/profile/reading/tok-1/library')

    // The inline library (search + shelf sidebar) now lives on its own route.
    expect(screen.queryByPlaceholderText('Search books…')).not.toBeInTheDocument()
    expect(screen.queryByText('custom-shelf')).not.toBeInTheDocument()
  })

  it('shows the currently-reading strip only', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksClient token="tok-1" />)

    // Reading Book appears once (currently-reading strip); library-only books
    // like the wishlist entry are not shown on the dashboard.
    expect(screen.getAllByText('Reading Book')).toHaveLength(1)
    expect(screen.queryByText('Wishlist Book')).not.toBeInTheDocument()
  })

  it('is read-only: no refresh button', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksClient token="tok-1" />)

    expect(screen.queryByRole('button', { name: /refresh/i })).not.toBeInTheDocument()
  })

  it('shows an error state when the library fails to load', () => {
    mockUseSharedLibrary.mockReturnValue({ data: undefined, error: new Error('nope') })
    render(<ProfileBooksClient token="tok-1" />)

    expect(screen.getByText('Failed to load books.')).toBeInTheDocument()
  })

  it('switches to the all-time chart and loads progress data', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    mockUseSharedBooksProgress.mockReturnValue({
      data: {
        progress: {
          labels: ['2026-01-01', '2026-01-02'],
          values: ['1', '2'],
          dateStart: '2026-01-01',
          dateEnd: '2026-01-02'
        }
      }
    })
    render(<ProfileBooksClient token="tok-1" />)

    fireEvent.click(screen.getByRole('tab', { name: 'All time' }))

    expect(screen.getByTestId('books-progress-chart')).toBeInTheDocument()
    expect(screen.getByLabelText('From')).toBeInTheDocument()
    expect(screen.getByLabelText('To')).toBeInTheDocument()
  })
})
