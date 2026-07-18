import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import ExternalBookCard from '@/components/reading/ExternalBookCard'
import { ExternalBookResultSchema } from '@/lib/gen/reading/v1/library_pb'

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

jest.mock('next/image', () => {
  return function MockImage({ src, alt }: { src: string; alt: string }) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} />
  }
})

const fakeBook = create(ExternalBookResultSchema, {
  provider: 'hardcover',
  providerId: '9780134190440',
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
    expect(screen.getByText('Hardcover')).toBeInTheDocument()
  })

  it('links to the external detail page', () => {
    render(<ExternalBookCard book={fakeBook} />)
    expect(screen.getByRole('link')).toHaveAttribute(
      'href',
      '/reading/external/hardcover/9780134190440'
    )
  })

  it('falls back to the raw provider string for unknown providers', () => {
    const other = create(ExternalBookResultSchema, { ...fakeBook, provider: 'unknownprovider' })
    render(<ExternalBookCard book={other} />)
    expect(screen.getByText('unknownprovider')).toBeInTheDocument()
  })

  it('omits the author line when there are no authors', () => {
    const noAuthors = create(ExternalBookResultSchema, { ...fakeBook, authors: [] })
    render(<ExternalBookCard book={noAuthors} />)
    expect(screen.queryByText('Alan Donovan, Brian Kernighan')).not.toBeInTheDocument()
  })

  // provider_id is the result's ISBN13 — a search result with no ISBN has no
  // detail page to link to (both configured providers only fetch by ISBN).
  it('renders as a non-clickable card when providerId is empty', () => {
    const noProviderId = create(ExternalBookResultSchema, { ...fakeBook, providerId: '' })
    render(<ExternalBookCard book={noProviderId} />)
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    expect(screen.getByText('The Go Programming Language')).toBeInTheDocument()
  })
})
