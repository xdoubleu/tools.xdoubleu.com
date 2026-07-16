import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/profile/ProfileLanding', () => () => <div data-testid="profile-landing" />)

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

import ProfilePage, { metadata } from '@/app/profile/[token]/page'

describe('ProfilePage', () => {
  it('renders the shared profile heading and landing component', async () => {
    render(await ProfilePage({ params: Promise.resolve({ token: 'tok-1' }) }))
    expect(screen.getByRole('heading', { name: 'Shared profile' })).toBeInTheDocument()
    expect(screen.getByTestId('profile-landing')).toBeInTheDocument()
  })

  it('is excluded from search indexing', () => {
    expect(metadata.robots).toEqual({ index: false, follow: false })
  })
})
