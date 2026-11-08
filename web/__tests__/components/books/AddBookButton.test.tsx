import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'

let currentSearchParams = new URLSearchParams()

vi.mock('@/hooks/navigation', () => ({
  useSearchParams: () => currentSearchParams
}))

const mockCreateBook = vi.fn()

// AddManualBookDialog calls useLibrary; stub a minimal shape.
vi.mock('@/hooks/useBooks', () => ({
  useCreateBook: () => mockCreateBook,
  useLibrary: () => ({
    data: { library: { shelves: [] } },
    isLoading: false,
    error: undefined
  })
}))

import AddBookButton from '@/components/books/AddBookButton'

describe('AddBookButton', () => {
  beforeEach(() => {
    currentSearchParams = new URLSearchParams()
  })

  it('opens the manual-add dialog prefilled with the search query', () => {
    currentSearchParams = new URLSearchParams('q=  Dungeon Crawler Carl ')
    render(<AddBookButton />)
    fireEvent.click(screen.getByRole('button', { name: 'Add book' }))
    expect(screen.getByRole('heading', { name: 'Add book manually' })).toBeInTheDocument()
    expect(screen.getByLabelText('Title')).toHaveValue('Dungeon Crawler Carl')
  })

  it('opens with an empty title when there is no search', () => {
    render(<AddBookButton />)
    fireEvent.click(screen.getByRole('button', { name: 'Add book' }))
    expect(screen.getByLabelText('Title')).toHaveValue('')
  })

  it('closes the manual-add dialog via its Cancel button', () => {
    render(<AddBookButton />)
    fireEvent.click(screen.getByRole('button', { name: 'Add book' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('heading', { name: 'Add book manually' })).not.toBeInTheDocument()
  })
})
