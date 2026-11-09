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
})
