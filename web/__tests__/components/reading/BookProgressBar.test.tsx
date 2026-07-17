import React from 'react'
import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/reading/v1/library_pb'
import BookProgressBar from '@/components/reading/BookProgressBar'

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
})
