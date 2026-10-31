import React from 'react'
import { render, screen } from '@testing-library/react'

vi.mock('@/components/factory/AgentsClient', () => ({
  __esModule: true,
  default: () => <div data-testid="agents" />
}))

import FactoryAgentsPage from '@/app/factory/agents/page'

describe('FactoryAgentsPage', () => {
  it('renders the agent accounts', () => {
    render(<FactoryAgentsPage />)
    expect(screen.getByTestId('agents')).toBeInTheDocument()
  })
})
