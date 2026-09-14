import React from 'react'
import { render, screen } from '@testing-library/react'

const mockUseAutomatedActions = jest.fn()
jest.mock('@/hooks/useMonitoring', () => ({
  useAutomatedActions: () => mockUseAutomatedActions()
}))

jest.mock('@/components/monitoring/AutomatedActionsCard', () => ({
  __esModule: true,
  default: ({ data }: { data?: unknown }) => (
    <div data-testid="automated-actions-card">{data ? 'has-data' : 'no-data'}</div>
  )
}))

import ObservabilityClient from '@/components/monitoring/ObservabilityClient'

describe('ObservabilityClient', () => {
  it('passes the automated actions SWR data through to the card', () => {
    mockUseAutomatedActions.mockReturnValue({ data: { actions: [] } })
    render(<ObservabilityClient />)
    expect(screen.getByTestId('automated-actions-card')).toHaveTextContent('has-data')
  })

  it('renders without data while loading', () => {
    mockUseAutomatedActions.mockReturnValue({ data: undefined })
    render(<ObservabilityClient />)
    expect(screen.getByTestId('automated-actions-card')).toHaveTextContent('no-data')
  })
})
