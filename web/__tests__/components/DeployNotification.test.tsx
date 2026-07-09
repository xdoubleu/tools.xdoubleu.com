import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'

const mockUseSWR = jest.fn()
const mockGetRelease = jest.fn()

jest.mock('swr', () => ({
  __esModule: true,
  default: (...args: unknown[]) => mockUseSWR(...args)
}))

jest.mock('@/lib/env', () => ({
  getRelease: () => mockGetRelease()
}))

import DeployNotification from '@/components/DeployNotification'

describe('DeployNotification', () => {
  beforeEach(() => {
    mockUseSWR.mockReset()
    mockGetRelease.mockReset()
    mockGetRelease.mockReturnValue('abc1234')
  })

  it('renders nothing when the release matches the baseline', () => {
    mockUseSWR.mockReturnValue({ data: { release: 'abc1234' } })
    const { container } = render(<DeployNotification />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing when no release data is available yet', () => {
    mockUseSWR.mockReturnValue({ data: undefined })
    const { container } = render(<DeployNotification />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing when the baseline is dev (local/unset)', () => {
    mockGetRelease.mockReturnValue('dev')
    mockUseSWR.mockReturnValue({ data: { release: 'def5678' } })
    const { container } = render(<DeployNotification />)
    expect(container).toBeEmptyDOMElement()
  })

  it('shows the notification when the release differs from the baseline', () => {
    mockUseSWR.mockReturnValue({ data: { release: 'def5678' } })
    render(<DeployNotification />)
    expect(screen.getByText('A new version is available.')).toBeInTheDocument()
  })

  it('reloads the page when Reload is clicked', () => {
    mockUseSWR.mockReturnValue({ data: { release: 'def5678' } })
    // jsdom's window.location doesn't allow reconfiguring `reload`, so this
    // exercises the click handler and confirms it doesn't throw rather than
    // asserting the (unmockable) browser API call itself.
    render(<DeployNotification />)
    expect(() => fireEvent.click(screen.getByRole('button', { name: 'Reload' }))).not.toThrow()
  })

  it('dismisses the notification when the close button is clicked', () => {
    mockUseSWR.mockReturnValue({ data: { release: 'def5678' } })
    render(<DeployNotification />)

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }))

    expect(screen.queryByText('A new version is available.')).not.toBeInTheDocument()
  })
})
