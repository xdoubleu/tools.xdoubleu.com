import React from 'react'
import { render, screen } from '@testing-library/react'

vi.mock('@/components/user-management/AgentsClient', () => ({
  __esModule: true,
  default: () => <div data-testid="agents" />
}))

import UserManagementAgentsPage from '@/app/user-management/agents/page'

describe('UserManagementAgentsPage', () => {
  it('renders the agent accounts', () => {
    render(<UserManagementAgentsPage />)
    expect(screen.getByTestId('agents')).toBeInTheDocument()
  })
})
