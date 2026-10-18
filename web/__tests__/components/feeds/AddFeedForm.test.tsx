import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConnectError, Code } from '@connectrpc/connect'

const createFeed = jest.fn()
jest.mock('@/hooks/useFeeds', () => ({
  useCreateFeed: () => createFeed
}))
jest.mock('@/lib/gen/feeds/v1/feeds_pb', () => ({
  FeedKind: { RSS: 1, EMAIL: 2, SCRAPE: 3 }
}))

import AddFeedForm from '@/components/feeds/AddFeedForm'

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

  it('shows a friendly message for an invalid feed URL', async () => {
    createFeed.mockRejectedValue(new ConnectError('bad', Code.InvalidArgument))
    render(<AddFeedForm />)

    fireEvent.change(screen.getByLabelText('Feed URL'), {
      target: { value: 'https://not-a-feed.example.com' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))

    await waitFor(() => {
      expect(
        screen.getByText('That URL did not work — check it and try again.')
      ).toBeInTheDocument()
    })
  })

  it('subscribes in scrape mode and reports that the import is in progress', async () => {
    createFeed.mockResolvedValue({})
    render(<AddFeedForm />)

    fireEvent.click(screen.getByLabelText('No RSS feed'))
    fireEvent.change(screen.getByLabelText('Feed URL'), {
      target: { value: 'https://blog.example.com' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))

    await waitFor(() => {
      expect(createFeed).toHaveBeenCalledWith('https://blog.example.com', 3, '')
      expect(
        screen.getByText('Subscribed — importing items in the background.')
      ).toBeInTheDocument()
    })
  })

  it('shows a scrape-mode hint before submitting', () => {
    render(<AddFeedForm />)
    fireEvent.click(screen.getByLabelText('No RSS feed'))
    expect(screen.getByText(/Scans the page for post links/)).toBeInTheDocument()
  })

  it('shows a friendly message when email newsletters are not configured', async () => {
    createFeed.mockRejectedValue(new ConnectError('no email', Code.FailedPrecondition))
    render(<AddFeedForm />)

    fireEvent.click(screen.getByLabelText('Email newsletter'))
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))

    await waitFor(() => {
      expect(
        screen.getByText('Email newsletters are not configured on this server.')
      ).toBeInTheDocument()
    })
  })

  it('mints and displays a one-time inbound address for email mode', async () => {
    createFeed.mockResolvedValue({ feed: { inboundAddress: 'abc123@mail.example.com' } })
    render(<AddFeedForm />)

    fireEvent.click(screen.getByLabelText('Email newsletter'))
    fireEvent.change(screen.getByLabelText('Newsletter name'), {
      target: { value: 'My Substack' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))

    await waitFor(() => {
      expect(createFeed).toHaveBeenCalledWith('', 2, 'My Substack')
      expect(screen.getByDisplayValue('abc123@mail.example.com')).toBeInTheDocument()
    })
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
    createFeed.mockResolvedValue({ feed: { inboundAddress: 'abc123@mail.example.com' } })
    render(<AddFeedForm />)

    fireEvent.click(screen.getByLabelText('Email newsletter'))
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))
    await waitFor(() => {
      expect(screen.getByDisplayValue('abc123@mail.example.com')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: 'Copy' }))
    expect(writeText).toHaveBeenCalledWith('abc123@mail.example.com')
  })

  it('subscribes to an arXiv search with a generated URL and title', async () => {
    createFeed.mockResolvedValue({})
    render(<AddFeedForm />)

    fireEvent.click(screen.getByLabelText('arXiv papers'))
    const subscribe = screen.getByRole('button', { name: 'Subscribe' })
    expect(subscribe).toBeDisabled()

    fireEvent.change(screen.getByLabelText('arXiv categories'), {
      target: { value: 'cs.AI' }
    })
    fireEvent.change(screen.getByLabelText('Keywords'), { target: { value: 'agents' } })
    fireEvent.change(screen.getByLabelText('Author'), { target: { value: 'Turing' } })
    fireEvent.click(subscribe)

    await waitFor(() => {
      expect(createFeed).toHaveBeenCalledWith(
        expect.stringMatching(/^https:\/\/export\.arxiv\.org\/api\/query\?/),
        1,
        'arXiv: agents, Turing · cs.AI'
      )
      expect(screen.getByLabelText('arXiv categories')).toHaveValue('')
      expect(screen.getByLabelText('Keywords')).toHaveValue('')
      expect(screen.getByLabelText('Author')).toHaveValue('')
    })
  })

  it('shows the arXiv hint only in arXiv mode until a submit reports back', async () => {
    createFeed.mockResolvedValue({})
    render(<AddFeedForm />)
    const hint = /Only the 20 newest matches/
    expect(screen.queryByText(hint)).not.toBeInTheDocument()

    fireEvent.click(screen.getByLabelText('arXiv papers'))
    expect(screen.getByText(hint)).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('arXiv categories'), { target: { value: 'cs.AI' } })
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))
    await waitFor(() => {
      expect(screen.queryByText(hint)).not.toBeInTheDocument()
    })
  })

  it('starts the arXiv fields empty', () => {
    render(<AddFeedForm />)
    fireEvent.click(screen.getByLabelText('arXiv papers'))
    expect(screen.getByLabelText('arXiv categories')).toHaveValue('')
    expect(screen.getByLabelText('Keywords')).toHaveValue('')
    expect(screen.getByLabelText('Author')).toHaveValue('')
  })

  it('sends the trimmed URL and no title from RSS mode, ignoring other modes', async () => {
    createFeed.mockResolvedValue({})
    render(<AddFeedForm />)

    fireEvent.click(screen.getByLabelText('Email newsletter'))
    fireEvent.change(screen.getByLabelText('Newsletter name'), { target: { value: 'News' } })
    fireEvent.click(screen.getByLabelText('arXiv papers'))
    fireEvent.change(screen.getByLabelText('arXiv categories'), { target: { value: 'cs.AI' } })
    fireEvent.click(screen.getByLabelText('RSS feed'))
    fireEvent.change(screen.getByLabelText('Feed URL'), {
      target: { value: '  https://news.example.com/rss  ' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))

    await waitFor(() => {
      expect(createFeed).toHaveBeenCalledWith('https://news.example.com/rss', 1, '')
    })
  })

  it('sends no URL and the trimmed name from email mode', async () => {
    createFeed.mockResolvedValue({ feed: { inboundAddress: 'a@mail.example.com' } })
    render(<AddFeedForm />)

    fireEvent.change(screen.getByLabelText('Feed URL'), {
      target: { value: 'https://news.example.com/rss' }
    })
    fireEvent.click(screen.getByLabelText('Email newsletter'))
    fireEvent.change(screen.getByLabelText('Newsletter name'), {
      target: { value: '  News  ' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))

    await waitFor(() => {
      expect(createFeed).toHaveBeenCalledWith('', 2, 'News')
    })
  })

  it('does not submit a blank URL, even via the form', () => {
    render(<AddFeedForm />)
    fireEvent.change(screen.getByLabelText('Feed URL'), { target: { value: '   ' } })
    expect(screen.getByRole('button', { name: 'Subscribe' })).toBeDisabled()

    fireEvent.submit(screen.getByLabelText('Feed URL').closest('form')!)
    expect(createFeed).not.toHaveBeenCalled()
  })

  it('disables the submit button while the URL is empty in RSS mode', () => {
    render(<AddFeedForm />)
    expect(screen.getByRole('button', { name: 'Subscribe' })).toBeDisabled()
  })
})
