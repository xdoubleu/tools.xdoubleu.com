import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import ResourceLinkPicker from '@/components/learningpaths/ResourceLinkPicker'

const mockSearchLibrary = jest.fn()
const mockUseFeedItems = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useSearchLibrary: () => mockSearchLibrary
}))

jest.mock('@/hooks/useFeeds', () => ({
  useFeedItems: (...args: unknown[]) => mockUseFeedItems(...args)
}))

jest.useFakeTimers()

beforeEach(() => {
  jest.clearAllMocks()
  mockUseFeedItems.mockReturnValue({ data: { items: [] } })
})

afterEach(() => {
  jest.clearAllTimers()
})

describe('ResourceLinkPicker — unlinked', () => {
  it('renders the link buttons', () => {
    render(
      <ResourceLinkPicker onLinkBook={jest.fn()} onLinkFeedItem={jest.fn()} onUnlink={jest.fn()} />
    )
    expect(screen.getByRole('button', { name: 'Link a book' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Link a feed item' })).toBeInTheDocument()
  })
})

describe('ResourceLinkPicker — linked book', () => {
  it('shows the linked book and unlinks on click', () => {
    const onUnlink = jest.fn()
    render(
      <ResourceLinkPicker
        linkedBook={{ id: 'b1', title: 'The Go Programming Language' }}
        onLinkBook={jest.fn()}
        onLinkFeedItem={jest.fn()}
        onUnlink={onUnlink}
      />
    )
    expect(screen.getByText(/The Go Programming Language/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Unlink' }))
    expect(onUnlink).toHaveBeenCalled()
  })
})

describe('ResourceLinkPicker — linked feed item', () => {
  it('shows the linked feed item and unlinks on click', () => {
    const onUnlink = jest.fn()
    render(
      <ResourceLinkPicker
        linkedFeedItem={{ id: 'f1', title: 'An Interesting Article' }}
        onLinkBook={jest.fn()}
        onLinkFeedItem={jest.fn()}
        onUnlink={onUnlink}
      />
    )
    expect(screen.getByText(/An Interesting Article/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Unlink' }))
    expect(onUnlink).toHaveBeenCalled()
  })

  it('prefers the linked book branch when both are somehow set', () => {
    render(
      <ResourceLinkPicker
        linkedBook={{ id: 'b1', title: 'Book Wins' }}
        linkedFeedItem={{ id: 'f1', title: 'Feed Loses' }}
        onLinkBook={jest.fn()}
        onLinkFeedItem={jest.fn()}
        onUnlink={jest.fn()}
      />
    )
    expect(screen.getByText(/Book Wins/)).toBeInTheDocument()
    expect(screen.queryByText(/Feed Loses/)).not.toBeInTheDocument()
  })
})

describe('ResourceLinkPicker — book search', () => {
  it('searches the library after debounce and picks a result', async () => {
    const onLinkBook = jest.fn()
    mockSearchLibrary.mockResolvedValue({
      books: [{ bookId: 'b1', book: { title: 'Dune' } }]
    })
    render(
      <ResourceLinkPicker onLinkBook={onLinkBook} onLinkFeedItem={jest.fn()} onUnlink={jest.fn()} />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Link a book' }))
    fireEvent.change(screen.getByPlaceholderText('Search your library…'), {
      target: { value: 'Dune' }
    })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })
    await waitFor(() => screen.getByText('Dune'))
    fireEvent.click(screen.getByText('Dune'))
    expect(onLinkBook).toHaveBeenCalledWith({ id: 'b1', title: 'Dune' })
    // Picking a result returns to the unlinked view.
    expect(screen.getByRole('button', { name: 'Link a book' })).toBeInTheDocument()
  })

  it('falls back to (untitled) when a hit has no book title', async () => {
    mockSearchLibrary.mockResolvedValue({
      books: [{ bookId: 'b1', book: undefined }]
    })
    render(
      <ResourceLinkPicker onLinkBook={jest.fn()} onLinkFeedItem={jest.fn()} onUnlink={jest.fn()} />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Link a book' }))
    fireEvent.change(screen.getByPlaceholderText('Search your library…'), {
      target: { value: 'Dune' }
    })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })
    await waitFor(() => screen.getByText('(untitled)'))
  })

  it('clears hits when the query is emptied', async () => {
    mockSearchLibrary.mockResolvedValue({
      books: [{ bookId: 'b1', book: { title: 'Dune' } }]
    })
    render(
      <ResourceLinkPicker onLinkBook={jest.fn()} onLinkFeedItem={jest.fn()} onUnlink={jest.fn()} />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Link a book' }))
    const input = screen.getByPlaceholderText('Search your library…')
    fireEvent.change(input, { target: { value: 'Dune' } })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })
    await waitFor(() => screen.getByText('Dune'))
    fireEvent.change(input, { target: { value: '' } })
    expect(screen.queryByText('Dune')).not.toBeInTheDocument()
  })

  it('shows no results when the search rejects', async () => {
    mockSearchLibrary.mockRejectedValue(new Error('boom'))
    render(
      <ResourceLinkPicker onLinkBook={jest.fn()} onLinkFeedItem={jest.fn()} onUnlink={jest.fn()} />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Link a book' }))
    fireEvent.change(screen.getByPlaceholderText('Search your library…'), {
      target: { value: 'Dune' }
    })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })
    await waitFor(() => expect(mockSearchLibrary).toHaveBeenCalled())
    expect(screen.queryByRole('listitem')).not.toBeInTheDocument()
  })

  it('cancels back to the unlinked view', () => {
    render(
      <ResourceLinkPicker onLinkBook={jest.fn()} onLinkFeedItem={jest.fn()} onUnlink={jest.fn()} />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Link a book' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.getByRole('button', { name: 'Link a book' })).toBeInTheDocument()
  })
})

describe('ResourceLinkPicker — feed item search', () => {
  it('filters cached feed items client-side and picks a result', () => {
    const onLinkFeedItem = jest.fn()
    mockUseFeedItems.mockReturnValue({
      data: {
        items: [
          { id: 'f1', title: 'Learning Go' },
          { id: 'f2', title: 'Cooking Pasta' }
        ]
      }
    })
    render(
      <ResourceLinkPicker
        onLinkBook={jest.fn()}
        onLinkFeedItem={onLinkFeedItem}
        onUnlink={jest.fn()}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Link a feed item' }))
    fireEvent.change(screen.getByPlaceholderText('Search your feed items…'), {
      target: { value: 'go' }
    })
    expect(screen.getByText('Learning Go')).toBeInTheDocument()
    expect(screen.queryByText('Cooking Pasta')).not.toBeInTheDocument()
    fireEvent.click(screen.getByText('Learning Go'))
    expect(onLinkFeedItem).toHaveBeenCalledWith({ id: 'f1', title: 'Learning Go' })
    expect(screen.getByRole('button', { name: 'Link a feed item' })).toBeInTheDocument()
  })

  it('shows nothing until a query is entered', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [{ id: 'f1', title: 'Learning Go' }] }
    })
    render(
      <ResourceLinkPicker onLinkBook={jest.fn()} onLinkFeedItem={jest.fn()} onUnlink={jest.fn()} />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Link a feed item' }))
    expect(screen.queryByText('Learning Go')).not.toBeInTheDocument()
  })

  it('handles an undefined items list', () => {
    mockUseFeedItems.mockReturnValue({ data: undefined })
    render(
      <ResourceLinkPicker onLinkBook={jest.fn()} onLinkFeedItem={jest.fn()} onUnlink={jest.fn()} />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Link a feed item' }))
    fireEvent.change(screen.getByPlaceholderText('Search your feed items…'), {
      target: { value: 'go' }
    })
    expect(screen.queryByRole('listitem')).not.toBeInTheDocument()
  })

  it('cancels back to the unlinked view', () => {
    render(
      <ResourceLinkPicker onLinkBook={jest.fn()} onLinkFeedItem={jest.fn()} onUnlink={jest.fn()} />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Link a feed item' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.getByRole('button', { name: 'Link a feed item' })).toBeInTheDocument()
  })
})
