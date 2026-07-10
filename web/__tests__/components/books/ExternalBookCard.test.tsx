import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import ExternalBookCard from '@/components/books/ExternalBookCard'
import { ExternalBookResultSchema } from '@/lib/gen/books/v1/library_pb'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('next/image', () => {
  return function MockImage({ src, alt }: { src: string; alt: string }) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} />
  }
})

const fakeBook = create(ExternalBookResultSchema, {
  provider: 'openlibrary',
  providerId: 'OL123W',
  title: 'The Go Programming Language',
  authors: ['Alan Donovan', 'Brian Kernighan'],
  isbn13: '9780134190440',
  coverUrl: '',
  description: ''
})

describe('ExternalBookCard', () => {
  it('renders title and authors', () => {
    render(<ExternalBookCard book={fakeBook} />)
    expect(screen.getByText('The Go Programming Language')).toBeInTheDocument()
    expect(screen.getByText('Alan Donovan, Brian Kernighan')).toBeInTheDocument()
  })

  it('shows the provider as a badge', () => {
    render(<ExternalBookCard book={fakeBook} />)
    expect(screen.getByText('OpenLibrary')).toBeInTheDocument()
  })

  it('links to the external detail page', () => {
    render(<ExternalBookCard book={fakeBook} />)
    expect(screen.getByRole('link')).toHaveAttribute('href', '/books/external/openlibrary/OL123W')
  })

  it('falls back to the raw provider string for unknown providers', () => {
    const other = create(ExternalBookResultSchema, { ...fakeBook, provider: 'googlebooks' })
    render(<ExternalBookCard book={other} />)
    expect(screen.getByText('googlebooks')).toBeInTheDocument()
  })

  it('omits the author line when there are no authors', () => {
    const noAuthors = create(ExternalBookResultSchema, { ...fakeBook, authors: [] })
    render(<ExternalBookCard book={noAuthors} />)
    expect(screen.queryByText('Alan Donovan, Brian Kernighan')).not.toBeInTheDocument()
  })
})
