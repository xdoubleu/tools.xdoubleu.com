import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'

const mockCreate = vi.fn()
const mockUpdate = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryActions: vi.fn(() => ({ createRepo: mockCreate, updateRepo: mockUpdate }))
}))

import RepoDialog from '@/components/factory/RepoDialog'
import { Harness, RepoSchema } from '@/lib/gen/factory/v1/factory_pb'

beforeEach(() => {
  vi.clearAllMocks()
  mockCreate.mockResolvedValue(undefined)
  mockUpdate.mockResolvedValue(undefined)
})

function fill(label: string, value: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value } })
}

describe('RepoDialog', () => {
  it('starts a new repo with the defaults and saves what is typed', async () => {
    const onClose = vi.fn()
    render(<RepoDialog onClose={onClose} />)
    expect(screen.getByText('Add repo')).toBeInTheDocument()
    expect(screen.getByLabelText('Owner')).toHaveValue('')
    expect(screen.getByLabelText('Name')).toHaveValue('')
    expect(screen.getByLabelText('Default branch')).toHaveValue('main')
    expect(screen.getByLabelText('Default harness')).toHaveValue(String(Harness.CLAUDE_CODE))
    expect(screen.getByLabelText('Time limit (minutes)')).toHaveValue(60)
    expect(screen.getByLabelText('Setup commands')).toHaveValue('')
    expect(screen.getByLabelText('Agent instructions')).toHaveValue('')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()

    fill('Owner', 'octo')
    fill('Name', 'cat')
    fill('Default branch', 'dev')
    fill('Default harness', String(Harness.OPENCODE))
    fill('Time limit (minutes)', '15')
    fill('Setup commands', 'npm ci')
    fill('Agent instructions', 'Be careful.')
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(mockCreate).toHaveBeenCalledWith({
        owner: 'octo',
        name: 'cat',
        defaultBranch: 'dev',
        defaultHarness: Harness.OPENCODE,
        setupCommands: 'npm ci',
        agentInstructions: 'Be careful.',
        timeLimitMinutes: 15
      })
    )
    await waitFor(() => expect(onClose).toHaveBeenCalled())
    expect(mockUpdate).not.toHaveBeenCalled()
  })

  it('edits an existing repo from its values', async () => {
    const repo = create(RepoSchema, {
      id: 'r1',
      owner: 'octo',
      name: 'cat',
      defaultBranch: 'trunk',
      defaultHarness: Harness.OPENCODE,
      setupCommands: 'make deps',
      agentInstructions: 'Follow AGENTS.md.',
      timeLimitMinutes: 90
    })
    const onClose = vi.fn()
    render(<RepoDialog repo={repo} onClose={onClose} />)

    expect(screen.getByText('Edit repo')).toBeInTheDocument()
    expect(screen.getByLabelText('Owner')).toHaveValue('octo')
    expect(screen.getByLabelText('Name')).toHaveValue('cat')
    expect(screen.getByLabelText('Default branch')).toHaveValue('trunk')
    expect(screen.getByLabelText('Default harness')).toHaveValue(String(Harness.OPENCODE))
    expect(screen.getByLabelText('Time limit (minutes)')).toHaveValue(90)
    expect(screen.getByLabelText('Setup commands')).toHaveValue('make deps')
    expect(screen.getByLabelText('Agent instructions')).toHaveValue('Follow AGENTS.md.')

    fill('Time limit (minutes)', '30')
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith(
        'r1',
        expect.objectContaining({ owner: 'octo', timeLimitMinutes: 30 })
      )
    )
    await waitFor(() => expect(onClose).toHaveBeenCalled())
    expect(mockCreate).not.toHaveBeenCalled()
  })

  it('keeps Save off for a blank owner or name', () => {
    render(<RepoDialog onClose={vi.fn()} />)
    fill('Owner', 'octo')
    fill('Name', '   ')
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
    fill('Owner', '   ')
    fill('Name', 'cat')
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
    fill('Owner', 'octo')
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
  })

  it('shows progress while saving', async () => {
    let finish: () => void = () => undefined
    mockCreate.mockReturnValue(new Promise<void>((resolve) => (finish = resolve)))
    render(<RepoDialog onClose={vi.fn()} />)
    fill('Owner', 'octo')
    fill('Name', 'cat')
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(await screen.findByRole('button', { name: 'Saving…' })).toBeDisabled()
    finish()
  })

  it('explains a duplicate, a validation message and any other failure', async () => {
    const onClose = vi.fn()
    render(<RepoDialog onClose={onClose} />)
    fill('Owner', 'octo')
    fill('Name', 'cat')
    const save = () => fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    mockCreate.mockRejectedValueOnce(new ConnectError('exists', Code.AlreadyExists))
    save()
    expect(await screen.findByRole('alert')).toHaveTextContent('That repo is already configured.')

    mockCreate.mockRejectedValueOnce(new ConnectError('invalid owner', Code.InvalidArgument))
    save()
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('invalid owner'))

    mockCreate.mockRejectedValueOnce(new Error('network'))
    save()
    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent(/Couldn.t save the repo/)
    )

    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
  })

  it('clears an earlier failure when saving again', async () => {
    mockCreate.mockRejectedValueOnce(new Error('network'))
    render(<RepoDialog onClose={vi.fn()} />)
    fill('Owner', 'octo')
    fill('Name', 'cat')
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await screen.findByRole('alert')

    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  })

  it('handles the form submit itself, without a page reload', () => {
    render(<RepoDialog onClose={vi.fn()} />)
    const form = screen.getByLabelText('Owner').closest('form')
    if (!form) throw new Error('no form')
    // fireEvent returns false when the handler called preventDefault.
    expect(fireEvent.submit(form)).toBe(false)
  })

  it('cancels or closes without saving', () => {
    const onClose = vi.fn()
    render(<RepoDialog onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(2)
    expect(mockCreate).not.toHaveBeenCalled()
  })

  it('only saves a whole-number time limit from 1 to 1440', () => {
    render(<RepoDialog onClose={vi.fn()} />)
    fill('Owner', 'octo')
    fill('Name', 'cat')
    const save = screen.getByRole('button', { name: 'Save' })
    expect(screen.queryByText('Enter a whole number from 1 to 1440.')).not.toBeInTheDocument()

    for (const bad of ['', '  ', '0', '-5', '1441', '1.5', 'abc']) {
      fill('Time limit (minutes)', bad)
      expect(save, bad).toBeDisabled()
      expect(screen.getByText('Enter a whole number from 1 to 1440.')).toBeInTheDocument()
    }
    for (const ok of ['1', '1440', '90']) {
      fill('Time limit (minutes)', ok)
      expect(save, ok).toBeEnabled()
    }
  })
})
