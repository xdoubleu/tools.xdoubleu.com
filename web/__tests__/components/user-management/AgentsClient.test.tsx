import React from 'react'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockRevoke = vi.fn()
const mockRotate = vi.fn()
const mockJoin = vi.fn()
const mockLeave = vi.fn()
const mockSetAccess = vi.fn()
vi.mock('@/hooks/useUserManagement', () => ({
  useAgents: vi.fn(),
  useAgentActions: vi.fn(() => ({
    revoke: mockRevoke,
    rotate: mockRotate,
    joinFamily: mockJoin,
    leaveFamily: mockLeave,
    setAccess: mockSetAccess
  }))
}))
vi.mock('@/components/user-management/AgentDialog', () => ({
  __esModule: true,
  default: ({ onClose }: { onClose: () => void }) => <div data-testid="dialog" onClick={onClose} />
}))

import AgentsClient from '@/components/user-management/AgentsClient'
import { useAgents } from '@/hooks/useUserManagement'
import {
  AgentAccountSchema,
  ListAgentAccountsResponseSchema
} from '@/lib/gen/factory/v1/agent_accounts_pb'

function mockAgents(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useAgents).mockReturnValue({ isLoading: false, ...value })
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

vi.mock('@/components/user-management/AgentStepUpDialog', () => ({
  __esModule: true,
  default: ({
    title,
    run,
    onClose
  }: {
    title: string
    run: (code: string) => Promise<unknown>
    onClose: () => void
  }) => (
    <div data-testid="step-up">
      <span>{title}</span>
      <button onClick={() => void run('123456')}>run</button>
      <button onClick={onClose}>close</button>
    </div>
  )
}))

beforeEach(() => {
  vi.clearAllMocks()
  mockRevoke.mockResolvedValue(undefined)
  mockRotate.mockResolvedValue(undefined)
  mockJoin.mockResolvedValue(undefined)
  mockLeave.mockResolvedValue(undefined)
  mockSetAccess.mockResolvedValue(undefined)
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
    expect(screen.getByRole('link', { name: 'User management' })).toHaveAttribute(
      'href',
      '/user-management'
    )
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
    expect(screen.queryByText(/Couldn.t complete that/)).not.toBeInTheDocument()
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
    expect(screen.queryByText(/Couldn.t complete that/)).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Revoke' }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Revoke' }))
    expect(await screen.findByText(/Couldn.t complete that/)).toBeInTheDocument()
  })

  it('shows progress while revoking and lets the user retry after a failure', async () => {
    let finish: () => void = () => undefined
    mockRevoke.mockReturnValueOnce(new Promise<void>((resolve) => (finish = resolve)))
    mockAgents({ data: list })
    render(<AgentsClient />)
    fireEvent.click(screen.getByRole('button', { name: 'Revoke' }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Revoke' }))
    expect(
      await within(screen.getByRole('dialog')).findByRole('button', { name: 'Revoking…' })
    ).toBeDisabled()
    finish()
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

    mockRevoke.mockRejectedValueOnce(new Error('network'))
    fireEvent.click(screen.getByRole('button', { name: 'Revoke' }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Revoke' }))
    await screen.findByText(/Couldn.t complete that/)
    expect(within(screen.getByRole('dialog')).getByRole('button', { name: 'Revoke' })).toBeEnabled()
  })

  it('shows each app at the level the agent holds it', () => {
    mockAgents({
      data: create(ListAgentAccountsResponseSchema, {
        accounts: [
          create(AgentAccountSchema, {
            id: 'a1',
            name: 'Reviewer',
            userId: 'u1',
            appAccess: ['factory', 'books'],
            writeAccess: ['factory']
          })
        ]
      })
    })
    render(<AgentsClient />)
    expect(screen.getByLabelText('factory access for Reviewer')).toHaveValue('write')
    expect(screen.getByLabelText('books access for Reviewer')).toHaveValue('read')
    expect(screen.getByLabelText('trains access for Reviewer')).toHaveValue('none')
  })

  it('changes a grant for the agent user, and says so when it fails', async () => {
    mockAgents({ data: list })
    render(<AgentsClient />)
    fireEvent.change(screen.getByLabelText('books access for Reviewer'), {
      target: { value: 'read' }
    })
    await waitFor(() => expect(mockSetAccess).toHaveBeenCalledWith('', 'books', 'read'))
    expect(screen.queryByText(/Couldn.t change the access/)).not.toBeInTheDocument()

    mockSetAccess.mockRejectedValueOnce(new Error('network'))
    fireEvent.change(screen.getByLabelText('feeds access for Reviewer'), {
      target: { value: 'write' }
    })
    expect(await screen.findByText(/Couldn.t change the access/)).toBeInTheDocument()
  })

  it('rotates the secret and joins the family behind the step-up sheet', async () => {
    mockAgents({ data: list })
    render(<AgentsClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Rotate secret' }))
    expect(screen.getByText("Rotate Reviewer's secret")).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'run' }))
    await waitFor(() => expect(mockRotate).toHaveBeenCalledWith('a1', '123456'))
    fireEvent.click(screen.getByRole('button', { name: 'close' }))
    expect(screen.queryByTestId('step-up')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Join my family' }))
    expect(screen.getByText('Add Reviewer to your family')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'run' }))
    await waitFor(() => expect(mockJoin).toHaveBeenCalledWith('a1', '123456'))
  })

  it('takes the agent out of the family, and says so when that fails', async () => {
    mockAgents({ data: list })
    render(<AgentsClient />)
    fireEvent.click(screen.getByRole('button', { name: 'Leave family' }))
    await waitFor(() => expect(mockLeave).toHaveBeenCalledWith('a1'))
    expect(screen.queryByText(/Couldn.t complete that/)).not.toBeInTheDocument()

    mockLeave.mockRejectedValueOnce(new Error('network'))
    fireEvent.click(screen.getByRole('button', { name: 'Leave family' }))
    expect(await screen.findByText(/Couldn.t complete that/)).toBeInTheDocument()
  })
})
