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

jest.mock('@/hooks/useTodoistConnection', () => ({
  useTodoistConnection: jest.fn(),
  useConnectTodoist: jest.fn(),
  useDisconnectTodoist: jest.fn()
}))

const mockSearchParams = new URLSearchParams()
jest.mock('next/navigation', () => ({
  useSearchParams: () => mockSearchParams
}))

function mockConnection({
  data,
  isLoading = false,
  mutate = jest.fn()
}: {
  data?: GetTodoistConnectionStatusResponse
  isLoading?: boolean
  mutate?: jest.Mock
}) {
  jest.mocked(useTodoistConnection).mockReturnValue({
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
    jest.mocked(useConnectTodoist).mockReturnValue(jest.fn().mockResolvedValue(undefined))
    jest.mocked(useDisconnectTodoist).mockReturnValue(jest.fn().mockResolvedValue({}))
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
    const connect = jest.fn().mockResolvedValue(undefined)
    jest.mocked(useConnectTodoist).mockReturnValue(connect)
    mockSearchParams.set('todoist_error', '1')
    mockConnection({ data: create(GetTodoistConnectionStatusResponseSchema, { connected: false }) })
    render(<LearningPathsSettingsClient />)
    expect(screen.getByRole('alert')).toHaveTextContent('Connecting Todoist failed')
    expect(screen.getByText('Not connected')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Connect' }))
    await waitFor(() => expect(connect).toHaveBeenCalled())
  })

  it('disconnects and revalidates when connected', async () => {
    const disconnect = jest.fn().mockResolvedValue({})
    const mutate = jest.fn()
    jest.mocked(useDisconnectTodoist).mockReturnValue(disconnect)
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
})
