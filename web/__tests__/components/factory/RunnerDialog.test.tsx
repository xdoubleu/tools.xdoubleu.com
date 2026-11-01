import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'

const mockCreate = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryActions: vi.fn(() => ({ createRunner: mockCreate }))
}))

import RunnerDialog from '@/components/factory/RunnerDialog'
import { CreateRunnerResponseSchema, RunnerSchema } from '@/lib/gen/factory/v1/runners_pb'

const created = create(CreateRunnerResponseSchema, {
  runner: create(RunnerSchema, { id: 'r1', name: 'Home Mac' }),
  pairingCode: 'ABCDE-FGHJK'
})

function fill(label: string, value: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value } })
}

beforeEach(() => {
  vi.clearAllMocks()
  mockCreate.mockResolvedValue(created)
})

describe('RunnerDialog', () => {
  it('starts empty with Create off until a name and code are given', () => {
    render(<RunnerDialog onClose={vi.fn()} />)
    expect(screen.getByText('Pair a runner')).toBeInTheDocument()
    expect(
      screen.getByText('The runner is the daemon on your Mac that will run jobs.')
    ).toBeInTheDocument()
    expect(screen.getByLabelText('Name')).toHaveValue('')
    expect(screen.getByLabelText('Authenticator code')).toHaveValue('')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()

    const button = screen.getByRole('button', { name: 'Create' })
    expect(button).toBeDisabled()
    fill('Name', 'Home Mac')
    expect(button).toBeDisabled()
    fill('Authenticator code', '   ')
    expect(button).toBeDisabled()
    fill('Authenticator code', '123456')
    expect(button).toBeEnabled()
    fill('Name', '  ')
    expect(button).toBeDisabled()
  })

  it('creates the runner with the trimmed code and shows the pairing code once', async () => {
    const onClose = vi.fn()
    render(<RunnerDialog onClose={onClose} />)
    fill('Name', 'Home Mac')
    fill('Authenticator code', ' 123456 ')
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() => expect(mockCreate).toHaveBeenCalledWith('Home Mac', '123456'))
    expect(await screen.findByText('Runner created')).toBeInTheDocument()
    expect(screen.getByText('Pair the Mac with this code.')).toBeInTheDocument()
    expect(screen.getByLabelText('Pairing code')).toHaveValue('ABCDE-FGHJK')
    expect(screen.getByText(/factory-runner pair ABCDE-FGHJK/)).toBeInTheDocument()
    expect(screen.getByText(/works once and expires in 10 minutes/)).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('keeps the code on screen until Done, whatever else is tried', async () => {
    const onClose = vi.fn()
    render(<RunnerDialog onClose={onClose} />)
    fill('Name', 'Home Mac')
    fill('Authenticator code', '1')
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await screen.findByText('Runner created')

    expect(screen.queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByLabelText('Pairing code')).toHaveValue('ABCDE-FGHJK')
  })

  it('shows progress while creating', async () => {
    let finish: (v: unknown) => void = () => undefined
    mockCreate.mockReturnValue(new Promise((resolve) => (finish = resolve)))
    render(<RunnerDialog onClose={vi.fn()} />)
    fill('Name', 'Home Mac')
    fill('Authenticator code', '1')
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    expect(await screen.findByRole('button', { name: 'Creating…' })).toBeDisabled()
    finish(created)
    await screen.findByText('Runner created')
  })

  it('explains each way creating can fail and stays open', async () => {
    const onClose = vi.fn()
    render(<RunnerDialog onClose={onClose} />)
    fill('Name', 'Home Mac')
    fill('Authenticator code', '1')
    const submit = () => fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    const alertText = (text: string | RegExp) =>
      waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(text))

    mockCreate.mockRejectedValueOnce(new ConnectError('gone', Code.Unauthenticated))
    submit()
    await alertText('Your session has expired. Sign in again.')

    mockCreate.mockRejectedValueOnce(new ConnectError('no', Code.PermissionDenied))
    submit()
    await alertText("That code didn't work, or no authenticator is set up for your account.")

    mockCreate.mockRejectedValueOnce(new ConnectError('slow', Code.ResourceExhausted))
    submit()
    await alertText('Too many attempts. Try again later.')

    mockCreate.mockRejectedValueOnce(new ConnectError('one', Code.AlreadyExists))
    submit()
    await alertText('A runner is already active. Revoke it before pairing another.')

    mockCreate.mockRejectedValueOnce(new ConnectError('name is bad', Code.InvalidArgument))
    submit()
    await alertText('name is bad')

    mockCreate.mockRejectedValueOnce(new Error('network'))
    submit()
    await alertText(/Couldn.t create the runner/)

    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Create' })).toBeEnabled()
  })

  it('hides an earlier failure while the next attempt is in flight', async () => {
    mockCreate.mockRejectedValueOnce(new Error('network'))
    let finish: (v: unknown) => void = () => undefined
    render(<RunnerDialog onClose={vi.fn()} />)
    fill('Name', 'Home Mac')
    fill('Authenticator code', '1')
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await screen.findByRole('alert')

    mockCreate.mockReturnValueOnce(new Promise((resolve) => (finish = resolve)))
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    expect(await screen.findByRole('button', { name: 'Creating…' })).toBeDisabled()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    finish(created)
    await screen.findByText('Runner created')
  })

  it('handles the form submit itself, and cancels or closes', () => {
    const onClose = vi.fn()
    render(<RunnerDialog onClose={onClose} />)
    const form = screen.getByLabelText('Name').closest('form')
    if (!form) throw new Error('no form')
    // fireEvent returns false when the handler called preventDefault.
    expect(fireEvent.submit(form)).toBe(false)

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(3)
  })
})
