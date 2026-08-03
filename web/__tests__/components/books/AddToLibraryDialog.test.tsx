import { render, screen, fireEvent } from '@testing-library/react'

jest.mock('@/components/books/BookSearchBar', () => {
  return function MockBookSearchBar() {
    return <div data-testid="book-search-bar" />
  }
})
jest.mock('@/components/books/AddByUrlForm', () => {
  return function MockAddByUrlForm() {
    return <div data-testid="add-by-url-form" />
  }
})
jest.mock('@/components/feeds/AddFeedForm', () => {
  return function MockAddFeedForm() {
    return <div data-testid="add-feed-form" />
  }
})

import AddToLibraryDialog from '@/components/books/AddToLibraryDialog'

describe('AddToLibraryDialog', () => {
  it('defaults to the book search mode', () => {
    render(<AddToLibraryDialog open onOpenChange={jest.fn()} />)
    expect(screen.getByTestId('book-search-bar')).toBeInTheDocument()
    expect(screen.queryByTestId('add-by-url-form')).not.toBeInTheDocument()
    expect(screen.queryByTestId('add-feed-form')).not.toBeInTheDocument()
  })

  it('switches to the By URL mode', () => {
    render(<AddToLibraryDialog open onOpenChange={jest.fn()} />)
    fireEvent.click(screen.getByRole('tab', { name: 'By URL' }))
    expect(screen.getByTestId('add-by-url-form')).toBeInTheDocument()
    expect(screen.queryByTestId('book-search-bar')).not.toBeInTheDocument()
  })

  it('switches to the RSS feed mode', () => {
    render(<AddToLibraryDialog open onOpenChange={jest.fn()} />)
    fireEvent.click(screen.getByRole('tab', { name: 'RSS feed' }))
    expect(screen.getByTestId('add-feed-form')).toBeInTheDocument()
  })
})
