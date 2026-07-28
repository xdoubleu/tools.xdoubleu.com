import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConnectError, Code } from '@connectrpc/connect'

const createFeed = jest.fn()
jest.mock('@/hooks/useBookFeeds', () => ({
  useCreateFeed: () => createFeed
}))
jest.mock('@/lib/gen/reading/v1/feeds_pb', () => ({
  FeedKind: { RSS: 1, EMAIL: 2 }
}))

import AddFeedForm from '@/components/reading/AddFeedForm'

describe('AddFeedForm', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('subscribes to a feed and reports that the import is in progress', async () => {
    createFeed.mockResolvedValue({})
    const onAdded = jest.fn()
    render(<AddFeedForm onAdded={onAdded} />)

    fireEvent.change(screen.getByLabelText('Feed URL'), {
      target: { value: 'https://news.example.com/rss' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))

    await waitFor(() => {
      expect(createFeed).toHaveBeenCalledWith('https://news.example.com/rss', 1, '')
      expect(
        screen.getByText('Subscribed — importing items in the background.')
      ).toBeInTheDocument()
      expect(onAdded).toHaveBeenCalled()
    })
  })

  it('shows a friendly message when already subscribed', async () => {
    createFeed.mockRejectedValue(new ConnectError('dup', Code.AlreadyExists))
    render(<AddFeedForm />)

    fireEvent.change(screen.getByLabelText('Feed URL'), {
      target: { value: 'https://news.example.com/rss' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))

    await waitFor(() => {
      expect(screen.getByText('You are already subscribed to this feed.')).toBeInTheDocument()
    })
  })

  it('mints and displays a one-time inbound address for email mode', async () => {
    createFeed.mockResolvedValue({ feed: { inboundAddress: 'reading+abc123@mail.example.com' } })
    render(<AddFeedForm />)

    fireEvent.click(screen.getByLabelText('Email newsletter'))
    fireEvent.change(screen.getByLabelText('Newsletter name'), {
      target: { value: 'My Substack' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))

    await waitFor(() => {
      expect(createFeed).toHaveBeenCalledWith('', 2, 'My Substack')
      expect(screen.getByDisplayValue('reading+abc123@mail.example.com')).toBeInTheDocument()
    })
    // The URL input is not applicable in email mode.
    expect(screen.queryByLabelText('Feed URL')).not.toBeInTheDocument()
  })

  it('shows a generic message for a non-ConnectError failure', async () => {
    createFeed.mockRejectedValue(new Error('network down'))
    render(<AddFeedForm />)

    fireEvent.change(screen.getByLabelText('Feed URL'), {
      target: { value: 'https://news.example.com/rss' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))

    await waitFor(() => {
      expect(screen.getByText('Subscribing failed. Please try again.')).toBeInTheDocument()
    })
  })

  it('copies the inbound address to the clipboard', async () => {
    const writeText = jest.fn()
    Object.assign(navigator, { clipboard: { writeText } })
    createFeed.mockResolvedValue({ feed: { inboundAddress: 'reading+abc123@mail.example.com' } })
    render(<AddFeedForm />)

    fireEvent.click(screen.getByLabelText('Email newsletter'))
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))
    await waitFor(() => {
      expect(screen.getByDisplayValue('reading+abc123@mail.example.com')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: 'Copy' }))
    expect(writeText).toHaveBeenCalledWith('reading+abc123@mail.example.com')
  })
})
