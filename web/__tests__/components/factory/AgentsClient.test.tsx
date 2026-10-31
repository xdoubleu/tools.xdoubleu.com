import React from 'react'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockRevoke = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryAgents: vi.fn(),
  useFactoryActions: vi.fn(() => ({ revokeAgent: mockRevoke }))
}))
vi.mock('@/components/factory/AgentDialog', () => ({
  __esModule: true,
  default: ({ onClose }: { onClose: () => void }) => <div data-testid="dialog" onClick={onClose} />
}))

import AgentsClient from '@/components/factory/AgentsClient'
import { useFactoryAgents } from '@/hooks/useFactory'
import {
  AgentAccountSchema,
  ListAgentAccountsResponseSchema
} from '@/lib/gen/factory/v1/agent_accounts_pb'

function mockAgents(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useFactoryAgents).mockReturnValue({ isLoading: false, ...value })
}

const list = create(ListAgentAccountsResponseSchema, {
  accounts: [
    create(AgentAccountSchema, { id: 'a1', name: 'Reviewer', clientId: 'agent-1' }),
    create(AgentAccountSchema, {
      id: 'a2',
      name: 'Old agent',
      clientId: 'agent-2',
      revokedAt: '2026-10-01T00:00:00Z'
    })
  ]
})

beforeEach(() => {
  vi.clearAllMocks()
  mockRevoke.mockResolvedValue(undefined)
})

describe('AgentsClient', () => {
  it('lists agents, with Revoke only for the active ones', () => {
    mockAgents({ data: list })
    render(<AgentsClient />)
    expect(screen.getByText('Reviewer')).toBeInTheDocument()
    expect(screen.getByText('agent-1')).toBeInTheDocument()
    expect(screen.getByText('Old agent')).toBeInTheDocument()
    expect(screen.getByText('Revoked')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Revoke' })).toHaveLength(1)
    expect(screen.getByRole('link', { name: 'Factory' })).toHaveAttribute('href', '/factory')
    expect(screen.getByText('Agents', { selector: '[aria-current="page"]' })).toBeInTheDocument()
  })

  it('shows loading, error and empty states', () => {
    mockAgents({ isLoading: true })
    const first = render(<AgentsClient />)
    expect(screen.getByText(/Loading agents/)).toBeInTheDocument()
    first.unmount()

    mockAgents({ error: new Error('boom') })
    const second = render(<AgentsClient />)
    expect(screen.getByText(/Failed to load agents/)).toBeInTheDocument()
    second.unmount()

    mockAgents({ data: create(ListAgentAccountsResponseSchema, {}) })
    render(<AgentsClient />)
    expect(screen.getByText('No agents yet.')).toBeInTheDocument()
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it('shows neither error nor spinner beside data', () => {
    mockAgents({ data: list, error: new Error('refetch'), isLoading: true })
    render(<AgentsClient />)
    expect(screen.queryByText(/Failed to load/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Loading agents/)).not.toBeInTheDocument()
    expect(screen.queryByText('No agents yet.')).not.toBeInTheDocument()
  })

  it('opens the create sheet and closes it again', () => {
    mockAgents({ data: list })
    render(<AgentsClient />)
    expect(screen.queryByTestId('dialog')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'New agent' }))
    expect(screen.getByTestId('dialog')).toBeInTheDocument()
    fireEvent.click(screen.getByTestId('dialog'))
    expect(screen.queryByTestId('dialog')).not.toBeInTheDocument()
  })

  it('asks before revoking, then revokes', async () => {
    mockAgents({ data: list })
    render(<AgentsClient />)
    expect(screen.queryByText('Revoke Reviewer?')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Revoke' }))
    expect(screen.getByText('Revoke Reviewer?')).toBeInTheDocument()
    expect(mockRevoke).not.toHaveBeenCalled()

    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Revoke' }))
    await waitFor(() => expect(mockRevoke).toHaveBeenCalledWith('a1'))
    await waitFor(() => expect(screen.queryByText('Revoke Reviewer?')).not.toBeInTheDocument())
    expect(screen.queryByText(/Couldn.t revoke/)).not.toBeInTheDocument()
  })

  it('cancels a revoke', () => {
    mockAgents({ data: list })
    render(<AgentsClient />)
    fireEvent.click(screen.getByRole('button', { name: 'Revoke' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByText('Revoke Reviewer?')).not.toBeInTheDocument()
    expect(mockRevoke).not.toHaveBeenCalled()
  })

  it('says so when revoking fails, and keeps the agent', async () => {
    mockRevoke.mockRejectedValueOnce(new Error('network'))
    mockAgents({ data: list })
    render(<AgentsClient />)
    expect(screen.queryByText(/Couldn.t revoke/)).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Revoke' }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Revoke' }))
    expect(await screen.findByText(/Couldn.t revoke the agent/)).toBeInTheDocument()
  })
})
