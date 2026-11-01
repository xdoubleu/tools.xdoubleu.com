import React from 'react'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockRevoke = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryRunners: vi.fn(),
  useFactoryActions: vi.fn(() => ({ revokeRunner: mockRevoke }))
}))
vi.mock('@/components/factory/RunnerDialog', () => ({
  __esModule: true,
  default: ({ onClose }: { onClose: () => void }) => <div data-testid="dialog" onClick={onClose} />
}))

import RunnersClient from '@/components/factory/RunnersClient'
import { useFactoryRunners } from '@/hooks/useFactory'
import { ListRunnersResponseSchema, RunnerSchema } from '@/lib/gen/factory/v1/runners_pb'

function mockRunners(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useFactoryRunners).mockReturnValue({ isLoading: false, ...value })
}

const list = create(ListRunnersResponseSchema, {
  runners: [
    create(RunnerSchema, {
      id: 'r1',
      name: 'Home Mac',
      version: '1.2.3',
      pairedAt: '2026-11-01T10:00:00Z',
      lastSeen: '2026-11-01T11:59:50Z',
      online: true
    }),
    create(RunnerSchema, {
      id: 'r0',
      name: 'Old Mac',
      version: '0.9.0',
      pairedAt: '2026-10-01T10:00:00Z',
      revokedAt: '2026-10-20T10:00:00Z'
    }),
    create(RunnerSchema, {
      id: 'r2',
      name: 'Waiting Mac',
      pairingExpiresAt: '2026-11-01T12:10:00Z'
    })
  ]
})

beforeEach(() => {
  vi.clearAllMocks()
  mockRevoke.mockResolvedValue(undefined)
})

describe('RunnersClient', () => {
  it('lists runners with their state, and Revoke only for the active ones', () => {
    mockRunners({ data: list })
    render(<RunnersClient />)
    expect(screen.getByText('Home Mac')).toBeInTheDocument()
    expect(screen.getByText('Version 1.2.3')).toBeInTheDocument()
    expect(screen.getByText('Online')).toBeInTheDocument()
    expect(screen.getByText('Revoked')).toBeInTheDocument()
    expect(screen.getByText('Waiting to pair')).toBeInTheDocument()
    expect(screen.getByText('Not paired yet')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Revoke' })).toHaveLength(2)
    expect(screen.getByText('Online').className).toContain('text-success')
    expect(screen.getByText('Revoked').className).toContain('text-subtle')
    expect(screen.queryByText(/Couldn't revoke/)).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Factory' })).toHaveAttribute('href', '/factory')
    expect(screen.getByText('Runner', { selector: '[aria-current="page"]' })).toBeInTheDocument()
  })

  it('shows loading, error and empty states', () => {
    mockRunners({ isLoading: true })
    const first = render(<RunnersClient />)
    expect(screen.getByText(/Loading runners/)).toBeInTheDocument()
    first.unmount()

    mockRunners({ error: new Error('boom') })
    const second = render(<RunnersClient />)
    expect(screen.getByText(/Failed to load runners/)).toBeInTheDocument()
    second.unmount()

    mockRunners({ data: create(ListRunnersResponseSchema, {}) })
    render(<RunnersClient />)
    expect(screen.getByText('No runner paired yet.')).toBeInTheDocument()
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it('shows neither error nor spinner beside data', () => {
    mockRunners({ data: list, error: new Error('refetch'), isLoading: true })
    render(<RunnersClient />)
    expect(screen.queryByText(/Failed to load/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Loading runners/)).not.toBeInTheDocument()
    expect(screen.queryByText('No runner paired yet.')).not.toBeInTheDocument()
  })

  it('opens the pairing sheet and closes it again', () => {
    mockRunners({ data: list })
    render(<RunnersClient />)
    expect(screen.queryByTestId('dialog')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Pair a runner' }))
    expect(screen.getByTestId('dialog')).toBeInTheDocument()
    fireEvent.click(screen.getByTestId('dialog'))
    expect(screen.queryByTestId('dialog')).not.toBeInTheDocument()
  })

  it('asks before revoking, then revokes', async () => {
    mockRunners({ data: list })
    render(<RunnersClient />)
    expect(screen.queryByText('Revoke Home Mac?')).not.toBeInTheDocument()

    fireEvent.click(screen.getAllByRole('button', { name: 'Revoke' })[0])
    expect(screen.getByText('Revoke Home Mac?')).toBeInTheDocument()
    expect(mockRevoke).not.toHaveBeenCalled()

    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Revoke' }))
    await waitFor(() => expect(mockRevoke).toHaveBeenCalledWith('r1'))
    await waitFor(() => expect(screen.queryByText('Revoke Home Mac?')).not.toBeInTheDocument())
    expect(screen.queryByText(/Couldn.t revoke/)).not.toBeInTheDocument()
  })

  it('cancels a revoke', () => {
    mockRunners({ data: list })
    render(<RunnersClient />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Revoke' })[0])
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByText('Revoke Home Mac?')).not.toBeInTheDocument()
    expect(mockRevoke).not.toHaveBeenCalled()
  })

  it('shows progress while revoking and lets the user retry after a failure', async () => {
    let finish: () => void = () => undefined
    mockRevoke.mockReturnValueOnce(new Promise<void>((resolve) => (finish = resolve)))
    mockRunners({ data: list })
    render(<RunnersClient />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Revoke' })[0])
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Revoke' }))
    expect(
      await within(screen.getByRole('dialog')).findByRole('button', { name: 'Revoking…' })
    ).toBeDisabled()
    finish()
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

    mockRevoke.mockRejectedValueOnce(new Error('network'))
    fireEvent.click(screen.getAllByRole('button', { name: 'Revoke' })[0])
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Revoke' }))
    expect(await screen.findByText(/Couldn.t revoke the runner/)).toBeInTheDocument()
    expect(within(screen.getByRole('dialog')).getByRole('button', { name: 'Revoke' })).toBeEnabled()
  })
})
