import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'

const mockClearKoboDeviceLogs = jest.fn()
const mockMutate = jest.fn()
const mockUseKoboDeviceLogs = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useKoboDeviceLogs: (id: string, enabled: boolean) => mockUseKoboDeviceLogs(id, enabled),
  useClearKoboDeviceLogs: () => mockClearKoboDeviceLogs
}))

import KoboDeviceLogs from '@/components/reading/KoboDeviceLogs'

const entry = {
  time: '2024-06-01T12:00:00Z',
  method: 'PUT',
  path: '/reading/kobo/tok/v1/library/abc/state',
  query: 'foo=bar',
  requestBody: '{"ReadingState":{"CurrentBookmark":{}}}',
  status: 200,
  responseBody: '{"CurrentBookmark":{}}'
}

beforeEach(() => {
  mockClearKoboDeviceLogs.mockReset()
  mockMutate.mockReset()
  mockClearKoboDeviceLogs.mockResolvedValue({})
})

it('shows loading state', () => {
  mockUseKoboDeviceLogs.mockReturnValue({ data: undefined, isLoading: true, mutate: mockMutate })
  render(<KoboDeviceLogs deviceId="dev-1" />)
  expect(screen.getByTestId('kobo-logs-loading')).toBeInTheDocument()
})

it('shows empty state when no entries', () => {
  mockUseKoboDeviceLogs.mockReturnValue({
    data: { entries: [] },
    isLoading: false,
    mutate: mockMutate
  })
  render(<KoboDeviceLogs deviceId="dev-1" />)
  expect(screen.getByTestId('kobo-logs-empty')).toBeInTheDocument()
})

it('renders captured entries with method, path, status and bodies', () => {
  mockUseKoboDeviceLogs.mockReturnValue({
    data: { entries: [entry] },
    isLoading: false,
    mutate: mockMutate
  })
  render(<KoboDeviceLogs deviceId="dev-1" />)
  expect(screen.getByTestId('kobo-logs-list')).toBeInTheDocument()
  expect(screen.getByText('PUT')).toBeInTheDocument()
  expect(screen.getByText(entry.path)).toBeInTheDocument()
  expect(screen.getByText(/200/)).toBeInTheDocument()
  expect(screen.getByText(entry.requestBody)).toBeInTheDocument()
  expect(screen.getByText(entry.responseBody)).toBeInTheDocument()
})

it('clears logs and revalidates on Clear', async () => {
  mockUseKoboDeviceLogs.mockReturnValue({
    data: { entries: [entry] },
    isLoading: false,
    mutate: mockMutate
  })
  render(<KoboDeviceLogs deviceId="dev-1" />)

  await act(async () => {
    fireEvent.click(screen.getByTestId('kobo-logs-clear'))
  })

  await waitFor(() => {
    expect(mockClearKoboDeviceLogs).toHaveBeenCalledWith('dev-1')
  })
  expect(mockMutate).toHaveBeenCalled()
})

it('disables Clear when there are no entries', () => {
  mockUseKoboDeviceLogs.mockReturnValue({
    data: { entries: [] },
    isLoading: false,
    mutate: mockMutate
  })
  render(<KoboDeviceLogs deviceId="dev-1" />)
  expect(screen.getByTestId('kobo-logs-clear')).toBeDisabled()
})
