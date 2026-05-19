import { render, screen, fireEvent } from '@testing-library/react'
import { PoliciesBanner } from '@/components/todos/PoliciesBanner'
import type { Policy } from '@/lib/gen/todos/v1/settings_pb'
import { clearPolicyBannerState } from '@/lib/todos/policiesBanner'

function makePolicy(overrides: Partial<Policy> = {}): Policy {
  return {
    id: 'policy-1',
    ownerUserId: 'user-1',
    text: 'Work for 25 min, rest for 5.',
    reappearAfterHours: 24,
    sortOrder: 0,
    createdAt: '2024-01-01T00:00:00Z',
    workspaceId: '',
    ...overrides
  } as Policy
}

beforeEach(() => {
  clearPolicyBannerState()
  localStorage.clear()
})

describe('PoliciesBanner', () => {
  it('renders nothing when policies list is empty', () => {
    render(<PoliciesBanner policies={[]} />)
    expect(screen.queryByRole('banner')).not.toBeInTheDocument()
  })

  it('renders the banner when a policy is provided and not dismissed', () => {
    render(<PoliciesBanner policies={[makePolicy()]} />)
    expect(screen.getByRole('banner')).toBeInTheDocument()
  })

  it('renders all policy texts', () => {
    const policies = [
      makePolicy({ id: 'p1', text: 'Policy A' }),
      makePolicy({ id: 'p2', text: 'Policy B' })
    ]
    render(<PoliciesBanner policies={policies} />)
    expect(screen.getByText('Policy A')).toBeInTheDocument()
    expect(screen.getByText('Policy B')).toBeInTheDocument()
  })

  it('hides the banner when dismissed', () => {
    render(<PoliciesBanner policies={[makePolicy()]} />)
    expect(screen.getByRole('banner')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /dismiss/i }))
    expect(screen.queryByRole('banner')).not.toBeInTheDocument()
  })

  it('stores the dismissal in localStorage on dismiss', () => {
    render(<PoliciesBanner policies={[makePolicy({ id: 'p-store' })]} />)
    fireEvent.click(screen.getByRole('button', { name: /dismiss/i }))
    expect(localStorage.length).toBe(1)
  })

  it('does not render when already dismissed', () => {
    const policy = makePolicy({ id: 'p-pre' })
    // Pre-dismiss it
    localStorage.setItem('policies:p-pre', String(Date.now()))
    render(<PoliciesBanner policies={[policy]} />)
    expect(screen.queryByRole('banner')).not.toBeInTheDocument()
  })

  it('renders dismiss button with accessible label', () => {
    render(<PoliciesBanner policies={[makePolicy()]} />)
    expect(screen.getByRole('button', { name: /dismiss policies banner/i })).toBeInTheDocument()
  })
})
