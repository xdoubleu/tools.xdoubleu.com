import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { FeedSchema } from '@/lib/gen/feeds/v1/feeds_pb'
import FeedExpirySelect from '@/components/feeds/FeedExpirySelect'

const update = vi.fn()
vi.mock('@/hooks/useFeedExpiry', () => ({ useUpdateFeedExpiry: () => update }))

const feed = (expiryDays: number) => create(FeedSchema, { id: 'feed-1', title: 'Blog', expiryDays })

describe('FeedExpirySelect', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    update.mockResolvedValue(undefined)
  })

  it('shows the feed lifetime with Never last', () => {
    render(<FeedExpirySelect feed={feed(7)} />)
    const select = screen.getByLabelText('Unread items expire after')
    expect(select).toHaveValue('7')
    const labels = Array.from(select.querySelectorAll('option')).map((o) => o.textContent)
    expect(labels).toEqual(['1 day', '3 days', '7 days', '14 days', '30 days', 'Never'])
  })

  it('keeps a lifetime set elsewhere selectable', () => {
    render(<FeedExpirySelect feed={feed(10)} />)
    expect(screen.getByLabelText('Unread items expire after')).toHaveValue('10')
    expect(screen.getByRole('option', { name: '10 days' })).toBeInTheDocument()
  })

  it('saves the chosen lifetime with the feed title', async () => {
    render(<FeedExpirySelect feed={feed(7)} />)
    fireEvent.change(screen.getByLabelText('Unread items expire after'), { target: { value: '0' } })
    await waitFor(() => expect(update).toHaveBeenCalledWith('feed-1', 'Blog', 0))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('announces a failed save', async () => {
    update.mockRejectedValue(new Error('nope'))
    render(<FeedExpirySelect feed={feed(7)} />)
    fireEvent.change(screen.getByLabelText('Unread items expire after'), { target: { value: '3' } })
    expect(await screen.findByRole('alert')).toHaveTextContent('Saving failed')
  })
})
