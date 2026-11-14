import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'

const mockSave = vi.fn()
const mockRun = vi.fn()
const mockStop = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryRoutines: vi.fn(),
  useFactoryActions: vi.fn(() => ({
    saveRoutine: mockSave,
    runRoutine: mockRun,
    stopRoutine: mockStop
  }))
}))

import RoutinesClient from '@/components/factory/RoutinesClient'
import { useFactoryRoutines } from '@/hooks/useFactory'
import {
  JobSchema,
  JobStatus,
  ListRoutinesResponseSchema,
  RoutineSchema
} from '@/lib/gen/factory/v1/factory_pb'

function mockRoutines(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useFactoryRoutines).mockReturnValue({ isLoading: false, ...value })
}

const list = create(ListRoutinesResponseSchema, {
  routines: [
    create(RoutineSchema, {
      name: 'noop',
      repoId: 'r1',
      enabled: false,
      intervalMinutes: 0,
      lastJob: create(JobSchema, {
        id: 'j1',
        status: JobStatus.SUCCEEDED,
        createdAt: '2026-11-01T10:00:00Z'
      })
    }),
    create(RoutineSchema, {
      name: 'sweep',
      repoId: 'r1',
      enabled: true,
      intervalMinutes: 45,
      lastJob: create(JobSchema, { id: 'j2', status: JobStatus.RUNNING })
    }),
    create(RoutineSchema, { name: 'fresh', repoId: 'r1' })
  ]
})

beforeEach(() => {
  vi.clearAllMocks()
  mockSave.mockResolvedValue(undefined)
  mockRun.mockResolvedValue(undefined)
  mockStop.mockResolvedValue(undefined)
})

describe('RoutinesClient', () => {
  it('lists each routine with its last run, status and Run or Stop', () => {
    mockRoutines({ data: list })
    render(<RoutinesClient />)
    expect(screen.getByText('noop')).toBeInTheDocument()
    expect(screen.getByText('Succeeded')).toBeInTheDocument()
    expect(screen.getByText('Running')).toBeInTheDocument()
    expect(screen.getByText('Never run')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Run noop' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Stop sweep' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Run sweep' })).not.toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Every 45 minutes' })).toBeInTheDocument()
  })

  it('runs, stops, enables and reschedules', async () => {
    mockRoutines({ data: list })
    render(<RoutinesClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Run noop' }))
    await waitFor(() => expect(mockRun).toHaveBeenCalledWith('noop'))
    fireEvent.click(screen.getByRole('button', { name: 'Stop sweep' }))
    await waitFor(() => expect(mockStop).toHaveBeenCalledWith('sweep'))

    fireEvent.click(screen.getAllByRole('button', { name: 'Enable' })[0])
    await waitFor(() =>
      expect(mockSave).toHaveBeenCalledWith({
        name: 'noop',
        repoId: 'r1',
        enabled: true,
        intervalMinutes: 0
      })
    )
    fireEvent.change(screen.getByLabelText('Schedule of noop'), { target: { value: '60' } })
    await waitFor(() =>
      expect(mockSave).toHaveBeenLastCalledWith({
        name: 'noop',
        repoId: 'r1',
        enabled: false,
        intervalMinutes: 60
      })
    )
  })

  it('says why a change was refused', async () => {
    mockRoutines({ data: list })
    mockRun.mockRejectedValueOnce(new ConnectError('no', Code.PermissionDenied))
    render(<RoutinesClient />)
    fireEvent.click(screen.getByRole('button', { name: 'Run noop' }))
    expect(await screen.findByText(/Only you, in the browser/)).toBeInTheDocument()

    mockRun.mockRejectedValueOnce(new ConnectError('busy', Code.AlreadyExists))
    fireEvent.click(screen.getByRole('button', { name: 'Run noop' }))
    expect(await screen.findByText(/already has a job/)).toBeInTheDocument()

    mockRun.mockRejectedValueOnce(new Error('boom'))
    fireEvent.click(screen.getByRole('button', { name: 'Run noop' }))
    expect(await screen.findByText(/Couldn't do that/)).toBeInTheDocument()
  })

  it('shows loading, error and empty states', () => {
    mockRoutines({ isLoading: true })
    const { rerender } = render(<RoutinesClient />)
    expect(screen.getByText('Loading routines…')).toBeInTheDocument()

    mockRoutines({ error: new Error('x') })
    rerender(<RoutinesClient />)
    expect(screen.getByText('Failed to load routines.')).toBeInTheDocument()

    mockRoutines({ data: create(ListRoutinesResponseSchema, {}) })
    rerender(<RoutinesClient />)
    expect(screen.getByText('No routines yet.')).toBeInTheDocument()
  })
})
