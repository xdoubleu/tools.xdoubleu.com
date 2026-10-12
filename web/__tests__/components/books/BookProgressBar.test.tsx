import React from 'react'
import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/books/v1/library_pb'
import BookProgressBar from '@/components/books/BookProgressBar'

describe('BookProgressBar', () => {
  it('shows pages label and fill width in pages mode', () => {
    const ub = create(UserBookSchema, {
      progressMode: 'pages',
      currentPage: 75,
      book: create(BookSchema, { pageCount: 300 })
    })
    render(<BookProgressBar userBook={ub} />)
    expect(screen.getByText('75 / 300 pages')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '25')
  })

  it('shows a percent label in percent mode', () => {
    const ub = create(UserBookSchema, { progressMode: 'percent', progressPercent: 40 })
    render(<BookProgressBar userBook={ub} />)
    expect(screen.getByText('40%')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '40')
  })

  it('falls back to a percent label when pages have no total', () => {
    const ub = create(UserBookSchema, {
      progressMode: 'pages',
      currentPage: 10,
      book: create(BookSchema, {})
    })
    render(<BookProgressBar userBook={ub} />)
    expect(screen.getByText('0%')).toBeInTheDocument()
  })

  it('uses the physical copy page count', () => {
    const ub = create(UserBookSchema, {
      progressMode: 'pages',
      currentPage: 50,
      physicalPageCount: 100,
      book: create(BookSchema, { pageCount: 300 })
    })
    render(<BookProgressBar userBook={ub} />)
    expect(screen.getByText('50 / 100 pages')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '50')
  })

  it('adds the physical page to a percent read digitally', () => {
    const ub = create(UserBookSchema, {
      progressMode: 'percent',
      progressPercent: 40,
      currentPage: 87,
      tags: ['own-physical'],
      formats: ['epub'],
      book: create(BookSchema, { pageCount: 300 })
    })
    render(<BookProgressBar userBook={ub} />)
    expect(screen.getByText('40% · p. ~87')).toBeInTheDocument()
  })

  it('leaves the page out without a page count to map digital positions', () => {
    const ub = create(UserBookSchema, {
      progressMode: 'percent',
      progressPercent: 40,
      currentPage: 87,
      tags: ['own-physical'],
      formats: ['epub']
    })
    render(<BookProgressBar userBook={ub} />)
    expect(screen.getByText('40%')).toBeInTheDocument()
  })

  it('leaves the page out without an EPUB to sync with', () => {
    const ub = create(UserBookSchema, {
      progressMode: 'percent',
      progressPercent: 40,
      currentPage: 87,
      tags: ['own-physical'],
      book: create(BookSchema, { pageCount: 300 })
    })
    render(<BookProgressBar userBook={ub} />)
    expect(screen.getByText('40%')).toBeInTheDocument()
  })

  it.each([
    ['without a physical copy', [], 87],
    ['before a page is known', ['own-physical'], 0]
  ])('leaves the page out %s', (_, tags, currentPage) => {
    const ub = create(UserBookSchema, {
      progressMode: 'percent',
      progressPercent: 40,
      currentPage,
      tags,
      formats: ['epub'],
      book: create(BookSchema, { pageCount: 300 })
    })
    render(<BookProgressBar userBook={ub} />)
    expect(screen.getByText('40%')).toBeInTheDocument()
  })
})
