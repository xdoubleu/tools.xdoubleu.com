import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConnectError, Code } from '@connectrpc/connect'

const createFeed = jest.fn()
jest.mock('@/hooks/useBookFeeds', () => ({
  useCreateFeed: () => createFeed
}))

import AddFeedForm from '@/components/reading/AddFeedForm'

describe('AddFeedForm', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('subscribes to a feed and reports the import count', async () => {
    createFeed.mockResolvedValue({ ingested: 3 })
    const onAdded = jest.fn()
    render(<AddFeedForm onAdded={onAdded} />)

    fireEvent.change(screen.getByLabelText('Feed URL'), {
      target: { value: 'https://news.example.com/rss' }
    })
    fireEvent.click(screen.getByRole('checkbox'))
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))

    await waitFor(() => {
      expect(createFeed).toHaveBeenCalledWith('https://news.example.com/rss', true)
      expect(screen.getByText(/imported 3 item/)).toBeInTheDocument()
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
})
