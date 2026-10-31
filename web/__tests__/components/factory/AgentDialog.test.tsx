import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'

const mockCreate = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryActions: vi.fn(() => ({ createAgent: mockCreate }))
}))

import AgentDialog from '@/components/factory/AgentDialog'
import {
  AgentAccountSchema,
  CreateAgentAccountResponseSchema
} from '@/lib/gen/factory/v1/agent_accounts_pb'

const created = create(CreateAgentAccountResponseSchema, {
  account: create(AgentAccountSchema, { id: 'a1', clientId: 'agent-abc' }),
  clientSecret: 'the-secret'
})

function fill(label: string, value: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value } })
}

let writeText: ReturnType<typeof vi.fn>

beforeEach(() => {
  vi.clearAllMocks()
  mockCreate.mockResolvedValue(created)
  writeText = vi.fn().mockResolvedValue(undefined)
  Object.assign(navigator, { clipboard: { writeText } })
})

describe('AgentDialog', () => {
  it('starts empty with Create off until a name and code are given', () => {
    render(<AgentDialog onClose={vi.fn()} />)
    expect(screen.getByText('New agent')).toBeInTheDocument()
    expect(screen.getByLabelText('Name')).toHaveValue('')
    expect(screen.getByLabelText('Authenticator code')).toHaveValue('')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()

    const create = screen.getByRole('button', { name: 'Create' })
    expect(create).toBeDisabled()
    fill('Name', 'Reviewer')
    expect(create).toBeDisabled()
    fill('Authenticator code', '   ')
    expect(create).toBeDisabled()
    fill('Authenticator code', '123456')
    expect(create).toBeEnabled()
    fill('Name', '  ')
    expect(create).toBeDisabled()
  })

  it('creates the agent with the trimmed code and shows the credentials once', async () => {
    const onClose = vi.fn()
    render(<AgentDialog onClose={onClose} />)
    fill('Name', 'Reviewer')
    fill('Authenticator code', ' 123456 ')
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() => expect(mockCreate).toHaveBeenCalledWith('Reviewer', '123456'))
    expect(await screen.findByText('Agent created')).toBeInTheDocument()
    expect(screen.getByLabelText('Client ID')).toHaveValue('agent-abc')
    expect(screen.getByLabelText('Client secret')).toHaveValue('the-secret')
    expect(screen.getByText(/can.t be shown again/)).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('copies the secret', async () => {
    render(<AgentDialog onClose={vi.fn()} />)
    fill('Name', 'Reviewer')
    fill('Authenticator code', '1')
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await screen.findByText('Agent created')
    expect(screen.getByRole('button', { name: 'Copy secret' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Copy secret' }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('the-secret'))
    expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument()
  })

  it('shows an empty client id when the response carries no account', async () => {
    mockCreate.mockResolvedValue(create(CreateAgentAccountResponseSchema, { clientSecret: 's' }))
    render(<AgentDialog onClose={vi.fn()} />)
    fill('Name', 'Reviewer')
    fill('Authenticator code', '1')
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await screen.findByText('Agent created')
    expect(screen.getByLabelText('Client ID')).toHaveValue('')
  })

  it('shows progress while creating', async () => {
    let finish: (v: unknown) => void = () => undefined
    mockCreate.mockReturnValue(new Promise((resolve) => (finish = resolve)))
    render(<AgentDialog onClose={vi.fn()} />)
    fill('Name', 'Reviewer')
    fill('Authenticator code', '1')
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    expect(await screen.findByRole('button', { name: 'Creating…' })).toBeDisabled()
    finish(created)
    await screen.findByText('Agent created')
  })

  it('explains each way creating can fail and stays open', async () => {
    const onClose = vi.fn()
    render(<AgentDialog onClose={onClose} />)
    fill('Name', 'Reviewer')
    fill('Authenticator code', '1')
    const submit = () => fireEvent.click(screen.getByRole('button', { name: 'Create' }))

    mockCreate.mockRejectedValueOnce(new ConnectError('no', Code.PermissionDenied))
    submit()
    expect(await screen.findByRole('alert')).toHaveTextContent("That code didn't work.")

    mockCreate.mockRejectedValueOnce(new ConnectError('slow', Code.ResourceExhausted))
    submit()
    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent('Too many attempts. Try again later.')
    )

    mockCreate.mockRejectedValueOnce(new ConnectError('name is bad', Code.InvalidArgument))
    submit()
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('name is bad'))

    mockCreate.mockRejectedValueOnce(new Error('network'))
    submit()
    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent(/Couldn.t create the agent/)
    )

    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Create' })).toBeEnabled()
  })

  it('clears an earlier failure when trying again', async () => {
    mockCreate.mockRejectedValueOnce(new Error('network'))
    render(<AgentDialog onClose={vi.fn()} />)
    fill('Name', 'Reviewer')
    fill('Authenticator code', '1')
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await screen.findByRole('alert')

    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  })

  it('handles the form submit itself, and cancels or closes', () => {
    const onClose = vi.fn()
    render(<AgentDialog onClose={onClose} />)
    const form = screen.getByLabelText('Name').closest('form')
    if (!form) throw new Error('no form')
    // fireEvent returns false when the handler called preventDefault.
    expect(fireEvent.submit(form)).toBe(false)

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(2)
  })
})
