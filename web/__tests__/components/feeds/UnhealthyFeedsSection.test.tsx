import React from 'react'
import { render, screen } from '@testing-library/react'
import UnhealthyFeedsSection from '@/components/feeds/UnhealthyFeedsSection'

const mockUseCurrentUser = jest.fn()
const mockUseUnhealthyFeeds = jest.fn()

jest.mock('@/hooks/useAuth', () => ({
  useCurrentUser: () => mockUseCurrentUser()
}))
jest.mock('@/hooks/useFeeds', () => ({
  useUnhealthyFeeds: (enabled: boolean) => mockUseUnhealthyFeeds(enabled)
}))

describe('UnhealthyFeedsSection', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUseUnhealthyFeeds.mockReturnValue({ data: undefined })
  })

  it('renders nothing for a non-admin viewer, and fetches nothing', () => {
    mockUseCurrentUser.mockReturnValue({ data: { role: 'user' } })
    const { container } = render(<UnhealthyFeedsSection />)
    expect(container).toBeEmptyDOMElement()
    expect(mockUseUnhealthyFeeds).toHaveBeenCalledWith(false)
  })

  it('renders nothing while the current user is unknown', () => {
    mockUseCurrentUser.mockReturnValue({ data: undefined })
    const { container } = render(<UnhealthyFeedsSection />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders the card and fetches for an admin viewer', () => {
    mockUseCurrentUser.mockReturnValue({ data: { role: 'admin' } })
    mockUseUnhealthyFeeds.mockReturnValue({ data: { feeds: [] } })
    render(<UnhealthyFeedsSection />)
    expect(screen.getByText('Unhealthy feeds')).toBeInTheDocument()
    expect(mockUseUnhealthyFeeds).toHaveBeenCalledWith(true)
  })
})
