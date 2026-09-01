import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import SharedFeedsCard from '@/components/dashboard/SharedFeedsCard'
import { SharedFeedSchema } from '@/lib/gen/dashboard/v1/reading_pb'

describe('SharedFeedsCard', () => {
  it('renders nothing with no feeds', () => {
    const { container } = render(<SharedFeedsCard feeds={[]} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing when feeds is undefined', () => {
    const { container } = render(<SharedFeedsCard />)
    expect(container).toBeEmptyDOMElement()
  })

  it('links feeds that have a public url and lists titles plainly otherwise', () => {
    render(
      <SharedFeedsCard
        feeds={[
          create(SharedFeedSchema, { title: 'Public feed', url: 'https://example.com/feed' }),
          create(SharedFeedSchema, { title: 'Email feed', url: '' })
        ]}
      />
    )
    const link = screen.getByRole('link', { name: 'Public feed' })
    expect(link).toHaveAttribute('href', 'https://example.com/feed')
    expect(screen.getByText('Email feed')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Email feed' })).not.toBeInTheDocument()
  })
})
