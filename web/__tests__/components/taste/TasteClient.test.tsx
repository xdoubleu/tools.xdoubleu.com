import React from 'react'
let mockListParam = ''
const mockReplace = jest.fn()
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), replace: mockReplace }),
  usePathname: () => '/movies',
  useSearchParams: () => new URLSearchParams(mockListParam)
}))

import { render, screen } from '@testing-library/react'

jest.mock('@/components/taste/TasteProfile', () => ({
  __esModule: true,
  default: () => <div data-testid="profile" />
}))
jest.mock('@/components/taste/TasteRecommendations', () => ({
  __esModule: true,
  default: () => <div data-testid="recommendations" />
}))
jest.mock('@/components/taste/TasteStats', () => ({
  __esModule: true,
  default: () => <div data-testid="stats" />
}))
jest.mock('@/components/taste/TasteEvents', () => ({
  __esModule: true,
  default: () => <div data-testid="events" />
}))

import TasteClient from '@/components/taste/TasteClient'

describe('TasteClient', () => {
  beforeEach(() => {
    mockListParam = ''
  })

  it('shows the profile and stats above the log', () => {
    render(<TasteClient />)
    expect(screen.getByTestId('profile')).toBeInTheDocument()
    expect(screen.getByTestId('recommendations')).toBeInTheDocument()
    expect(screen.getByTestId('stats')).toBeInTheDocument()
    expect(screen.getByText('Log')).toBeInTheDocument()
    expect(screen.getByTestId('events')).toBeInTheDocument()
  })

  it('switches to the family taste and says what it covers', () => {
    mockListParam = 'list=shared'
    render(<TasteClient />)
    expect(screen.getByRole('tab', { name: 'Family' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByText(/read as one person/)).toBeInTheDocument()
  })
})
