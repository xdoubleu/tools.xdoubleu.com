import type { Mock } from 'vitest'
import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import LearningPathsSettingsClient from '@/components/learningpaths/LearningPathsSettingsClient'
import {
  useTodoistConnection,
  useConnectTodoist,
  useDisconnectTodoist
} from '@/hooks/useTodoistConnection'
import {
  GetTodoistConnectionStatusResponseSchema,
  type GetTodoistConnectionStatusResponse
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'

vi.mock('@/hooks/useTodoistConnection', () => ({
  useTodoistConnection: vi.fn(),
  useConnectTodoist: vi.fn(),
  useDisconnectTodoist: vi.fn()
}))

const mockSearchParams = new URLSearchParams()
vi.mock('@/hooks/navigation', () => ({
  useSearchParams: () => mockSearchParams
}))

function mockConnection({
  data,
  isLoading = false,
  mutate = vi.fn()
}: {
  data?: GetTodoistConnectionStatusResponse
  isLoading?: boolean
  mutate?: Mock
}) {
  vi.mocked(useTodoistConnection).mockReturnValue({
    data,
    error: undefined,
    isLoading,
    isValidating: false,
    mutate
  })
}

describe('LearningPathsSettingsClient', () => {
  beforeEach(() => {
    mockSearchParams.delete('todoist_error')
    vi.mocked(useConnectTodoist).mockReturnValue(vi.fn().mockResolvedValue(undefined))
    vi.mocked(useDisconnectTodoist).mockReturnValue(vi.fn().mockResolvedValue({}))
  })

  it('shows a loading state', () => {
    mockConnection({ isLoading: true })
    render(<LearningPathsSettingsClient />)
    expect(
      screen.getByRole('heading', { level: 1, name: 'Learning Paths Settings' })
    ).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('Loading…')
  })

  it('connects when not connected and reports a failed connect', async () => {
    const connect = vi.fn().mockResolvedValue(undefined)
    vi.mocked(useConnectTodoist).mockReturnValue(connect)
    mockSearchParams.set('todoist_error', '1')
    mockConnection({ data: create(GetTodoistConnectionStatusResponseSchema, { connected: false }) })
    render(<LearningPathsSettingsClient />)
    expect(screen.getByRole('alert')).toHaveTextContent('Connecting Todoist failed')
    expect(screen.getByText('Not connected')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Connect' }))
    await waitFor(() => expect(connect).toHaveBeenCalled())
  })

  it('disconnects and revalidates when connected', async () => {
    const disconnect = vi.fn().mockResolvedValue({})
    const mutate = vi.fn()
    vi.mocked(useDisconnectTodoist).mockReturnValue(disconnect)
    mockConnection({
      data: create(GetTodoistConnectionStatusResponseSchema, {
        connected: true,
        connectedAt: '2026-01-02T03:04:05Z'
      }),
      mutate
    })
    render(<LearningPathsSettingsClient />)
    expect(screen.getByText(/^Connected on /)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Disconnect' }))
    await waitFor(() => expect(mutate).toHaveBeenCalled())
    expect(disconnect).toHaveBeenCalled()
  })

  it('shows an alert when connecting or disconnecting fails', async () => {
    vi.mocked(useConnectTodoist).mockReturnValue(vi.fn().mockRejectedValue(new Error('boom')))
    mockConnection({ data: create(GetTodoistConnectionStatusResponseSchema, { connected: false }) })
    const { unmount } = render(<LearningPathsSettingsClient />)
    fireEvent.click(screen.getByRole('button', { name: 'Connect' }))
    expect(await screen.findByRole('alert')).toHaveTextContent("Couldn't connect Todoist.")
    expect(screen.getByRole('button', { name: 'Connect' })).toBeEnabled()
    unmount()

    vi.mocked(useDisconnectTodoist).mockReturnValue(vi.fn().mockRejectedValue(new Error('boom')))
    mockConnection({ data: create(GetTodoistConnectionStatusResponseSchema, { connected: true }) })
    render(<LearningPathsSettingsClient />)
    fireEvent.click(screen.getByRole('button', { name: 'Disconnect' }))
    expect(await screen.findByRole('alert')).toHaveTextContent("Couldn't disconnect Todoist.")
  })

  it('asks to reconnect when the connection lacks scopes', async () => {
    const connect = vi.fn().mockResolvedValue(undefined)
    vi.mocked(useConnectTodoist).mockReturnValue(connect)
    mockConnection({
      data: create(GetTodoistConnectionStatusResponseSchema, {
        connected: true,
        connectedAt: '2026-01-02T03:04:05Z',
        needsReconnect: true
      })
    })
    render(<LearningPathsSettingsClient />)
    expect(screen.getByRole('status')).toHaveTextContent('Reconnect Todoist')
    fireEvent.click(screen.getByRole('button', { name: 'Reconnect' }))
    await waitFor(() => expect(connect).toHaveBeenCalled())
    expect(screen.getByRole('button', { name: 'Disconnect' })).toBeInTheDocument()
  })
})
