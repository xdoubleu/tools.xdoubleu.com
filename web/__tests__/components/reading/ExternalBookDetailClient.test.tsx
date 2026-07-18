import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import {
  ExternalBookResultSchema,
  GetExternalBookResponseSchema
} from '@/lib/gen/reading/v1/library_pb'

const mockAddBook = jest.fn().mockResolvedValue(undefined)

jest.mock('@/hooks/useBooks', () => ({
  useExternalBook: jest.fn(),
  useCreateBook: () => mockAddBook
}))

const mockRouterPush = jest.fn()
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockRouterPush })
}))

jest.mock('next/image', () => {
  return function MockImage({ src, alt }: { src: string; alt: string }) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} />
  }
})

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('swr', () => ({ mutate: jest.fn() }))

import ExternalBookDetailClient from '@/app/reading/external/[provider]/[providerId]/ExternalBookDetailClient'
import { useExternalBook } from '@/hooks/useBooks'

const mockBook = create(ExternalBookResultSchema, {
  provider: 'hardcover',
  providerId: '9780134190440',
  title: 'The Go Programming Language',
  authors: ['Alan Donovan', 'Brian Kernighan'],
  isbn13: '9780134190440',
  coverUrl: '',
  description: 'A great book about Go.'
})

function mockResult(book = mockBook) {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  jest.mocked(useExternalBook).mockReturnValue({
    data: create(GetExternalBookResponseSchema, { result: book }),
    error: undefined,
    isLoading: false
  })
}

describe('ExternalBookDetailClient', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('shows loading state', () => {
    // @ts-expect-error -- partial mock
    jest.mocked(useExternalBook).mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: true
    })
    render(<ExternalBookDetailClient provider="hardcover" providerId="9780134190440" />)
    expect(screen.getByText('Loading book…')).toBeInTheDocument()
  })

  it('shows error state', () => {
    // @ts-expect-error -- partial mock
    jest.mocked(useExternalBook).mockReturnValue({
      data: undefined,
      error: new Error('fail'),
      isLoading: false
    })
    render(<ExternalBookDetailClient provider="hardcover" providerId="9780134190440" />)
    expect(screen.getByText('Failed to load book.')).toBeInTheDocument()
  })

  it('shows not found when there is no result', () => {
    // @ts-expect-error -- partial mock
    jest.mocked(useExternalBook).mockReturnValue({
      data: create(GetExternalBookResponseSchema, {}),
      error: undefined,
      isLoading: false
    })
    render(<ExternalBookDetailClient provider="hardcover" providerId="9780134190440" />)
    expect(screen.getByText('Book not found.')).toBeInTheDocument()
  })

  it('renders title, authors, isbn and description', () => {
    mockResult()
    render(<ExternalBookDetailClient provider="hardcover" providerId="9780134190440" />)
    expect(screen.getByRole('heading', { name: 'The Go Programming Language' })).toBeInTheDocument()
    expect(screen.getByText('Alan Donovan, Brian Kernighan')).toBeInTheDocument()
    expect(screen.getByText('ISBN: 9780134190440')).toBeInTheDocument()
    expect(screen.getByText('A great book about Go.')).toBeInTheDocument()
    expect(screen.getByText('Hardcover')).toBeInTheDocument()
  })

  it('shows no description fallback when description is empty', () => {
    mockResult(create(ExternalBookResultSchema, { ...mockBook, description: '' }))
    render(<ExternalBookDetailClient provider="hardcover" providerId="9780134190440" />)
    expect(screen.getByText('No description available.')).toBeInTheDocument()
  })

  it('renders breadcrumb with Books and Library links', () => {
    mockResult()
    render(<ExternalBookDetailClient provider="hardcover" providerId="9780134190440" />)
    expect(screen.getByText('Reading').closest('a')).toHaveAttribute('href', '/reading')
    expect(screen.getByText('Library').closest('a')).toHaveAttribute('href', '/reading/library')
  })

  it('opens the add-to-library modal when "Add to library" is clicked', () => {
    mockResult()
    render(<ExternalBookDetailClient provider="hardcover" providerId="9780134190440" />)
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }))
    expect(screen.getByRole('button', { name: 'Add Book' })).toBeInTheDocument()
  })

  it('redirects to the library after the book is added', async () => {
    mockResult()
    render(<ExternalBookDetailClient provider="hardcover" providerId="9780134190440" />)
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }))
    fireEvent.click(screen.getByRole('button', { name: 'Add Book' }))
    await waitFor(() => {
      expect(mockRouterPush).toHaveBeenCalledWith('/reading/library')
    })
  })
})
