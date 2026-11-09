import React, { type ComponentProps } from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'

import AgentStepUpDialog from '@/components/user-management/AgentStepUpDialog'
import { AgentAccountSchema } from '@/lib/gen/factory/v1/agent_accounts_pb'

type Run = ComponentProps<typeof AgentStepUpDialog>['run']

function open(run: Run, onClose = vi.fn()) {
  render(
    <AgentStepUpDialog
      title="Do it"
      description="Needs a code."
      submitLabel="Go"
      run={run}
      onClose={onClose}
    />
  )
  return onClose
}

function enterCode(value: string) {
  fireEvent.change(screen.getByLabelText('Authenticator code'), { target: { value } })
}

describe('AgentStepUpDialog', () => {
  it('keeps the action off until a code is given', () => {
    open(vi.fn())
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Go' })).toBeDisabled()
    enterCode(' 123456 ')
    expect(screen.getByRole('button', { name: 'Go' })).toBeEnabled()
  })

  it('runs with the trimmed code and closes when nothing is issued', async () => {
    const run = vi.fn().mockResolvedValue(undefined)
    const onClose = open(run)
    enterCode(' 123456 ')
    fireEvent.click(screen.getByRole('button', { name: 'Go' }))
    await waitFor(() => expect(onClose).toHaveBeenCalled())
    expect(run).toHaveBeenCalledWith('123456')
  })

  it('shows an issued secret once and closes on Done', async () => {
    const run = vi.fn().mockResolvedValue({
      account: create(AgentAccountSchema, { clientId: 'agent-abc' }),
      clientSecret: 'fresh-secret'
    })
    const onClose = open(run)
    enterCode('123456')
    fireEvent.click(screen.getByRole('button', { name: 'Go' }))
    expect(await screen.findByDisplayValue('fresh-secret')).toBeInTheDocument()
    expect(screen.getByDisplayValue('agent-abc')).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('says why it failed and stays open', async () => {
    const run = vi.fn().mockRejectedValue(new ConnectError('nope', Code.PermissionDenied))
    const onClose = open(run)
    enterCode('000000')
    fireEvent.click(screen.getByRole('button', { name: 'Go' }))
    expect(await screen.findByText(/That code didn.t work/)).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('falls back to a generic message for other failures', async () => {
    open(vi.fn().mockRejectedValue(new Error('boom')))
    enterCode('123456')
    fireEvent.click(screen.getByRole('button', { name: 'Go' }))
    expect(await screen.findByText("That didn't work. Try again.")).toBeInTheDocument()
  })

  it('names the issued secret and drops the close button until Done', async () => {
    open(vi.fn().mockResolvedValue({ clientSecret: 'fresh-secret' }))
    expect(screen.getByRole('heading', { name: 'Do it' })).toBeInTheDocument()
    expect(screen.getByText('Needs a code.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Close' })).toBeInTheDocument()
    enterCode('123456')
    fireEvent.click(screen.getByRole('button', { name: 'Go' }))
    await screen.findByDisplayValue('fresh-secret')
    expect(screen.getByRole('heading', { name: 'New secret' })).toBeInTheDocument()
    expect(screen.getByText('The old secret keeps working for 24 hours.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
  })

  it('closes on Escape before a secret exists, but not once one is shown', async () => {
    const onClose = open(vi.fn().mockResolvedValue({ clientSecret: 'fresh-secret' }))
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)

    enterCode('123456')
    fireEvent.click(screen.getByRole('button', { name: 'Go' }))
    await screen.findByDisplayValue('fresh-secret')
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('shows progress while running, and clears an old failure on retry', async () => {
    let finish: (v: undefined) => void = () => undefined
    const run = vi
      .fn()
      .mockRejectedValueOnce(new Error('boom'))
      .mockImplementationOnce(() => new Promise((resolve) => (finish = resolve)))
    const onClose = open(run)
    enterCode('123456')
    fireEvent.click(screen.getByRole('button', { name: 'Go' }))
    await screen.findByText("That didn't work. Try again.")

    fireEvent.click(screen.getByRole('button', { name: 'Go' }))
    expect(await screen.findByRole('button', { name: 'Working…' })).toBeDisabled()
    expect(screen.queryByText("That didn't work. Try again.")).not.toBeInTheDocument()
    finish(undefined)
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('keeps the action off for a blank code', () => {
    open(vi.fn())
    enterCode('   ')
    expect(screen.getByRole('button', { name: 'Go' })).toBeDisabled()
  })

  it('handles the submit itself instead of reloading the page', async () => {
    const run = vi.fn().mockResolvedValue(undefined)
    const onClose = open(run)
    enterCode('123456')
    const form = screen.getByLabelText('Authenticator code').closest('form')
    expect(form).not.toBeNull()
    if (form) expect(fireEvent.submit(form)).toBe(false)
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })
})
