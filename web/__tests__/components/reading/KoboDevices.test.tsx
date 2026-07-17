import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'

const mockDisconnectKoboDevice = jest.fn()
const mockSetKoboDeviceLogging = jest.fn()
const mockMutate = jest.fn()

function makeUseListKoboDevices(devices: unknown[]) {
  return {
    data: { devices },
    isLoading: false,
    mutate: mockMutate
  }
}

const mockUseListKoboDevices = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useListKoboDevices: () => mockUseListKoboDevices(),
  useDisconnectKoboDevice: () => mockDisconnectKoboDevice,
  useSetKoboDeviceLogging: () => mockSetKoboDeviceLogging
}))

// Stub the logs viewer — it has its own test and its own hooks.
jest.mock('@/components/reading/KoboDeviceLogs', () => ({
  __esModule: true,
  default: ({ deviceId }: { deviceId: string }) => (
    <div data-testid={`kobo-logs-stub-${deviceId}`}>logs</div>
  )
}))

// Stub Dialog to render children inline (avoids portal issues in jsdom).
jest.mock('@/components/ui/dialog', () => ({
  Dialog: ({ children, open }: { children: React.ReactNode; open: boolean }) =>
    open ? <div data-testid="dialog">{children}</div> : null,
  DialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>
}))

import KoboDevices from '@/components/reading/KoboDevices'

beforeEach(() => {
  mockDisconnectKoboDevice.mockReset()
  mockSetKoboDeviceLogging.mockReset()
  mockMutate.mockReset()
  mockDisconnectKoboDevice.mockResolvedValue({})
  mockSetKoboDeviceLogging.mockResolvedValue({})
})

describe('KoboDevices — empty state', () => {
  it('shows empty message when no devices', () => {
    mockUseListKoboDevices.mockReturnValue(makeUseListKoboDevices([]))
    render(<KoboDevices />)
    expect(screen.getByTestId('kobo-devices-empty')).toBeInTheDocument()
  })

  it('shows loading state', () => {
    mockUseListKoboDevices.mockReturnValue({ data: undefined, isLoading: true, mutate: mockMutate })
    render(<KoboDevices />)
    expect(screen.getByTestId('kobo-devices-loading')).toBeInTheDocument()
  })
})

describe('KoboDevices — device list', () => {
  const device = {
    id: 'dev-abc',
    name: 'My Kobo',
    serial: 'N4181234',
    createdAt: '2024-01-01T00:00:00Z',
    lastSeenAt: '2024-06-01T12:00:00Z'
  }

  const deviceNoLastSeen = {
    id: 'dev-xyz',
    name: 'New Kobo',
    serial: '',
    createdAt: '2024-02-01T00:00:00Z',
    lastSeenAt: ''
  }

  it('renders a row for each device', () => {
    mockUseListKoboDevices.mockReturnValue(makeUseListKoboDevices([device, deviceNoLastSeen]))
    render(<KoboDevices />)
    expect(screen.getByTestId('kobo-devices-list')).toBeInTheDocument()
    expect(screen.getByTestId(`kobo-device-${device.id}`)).toBeInTheDocument()
    expect(screen.getByTestId(`kobo-device-${deviceNoLastSeen.id}`)).toBeInTheDocument()
  })

  it('shows device name', () => {
    mockUseListKoboDevices.mockReturnValue(makeUseListKoboDevices([device]))
    render(<KoboDevices />)
    expect(screen.getByText('My Kobo')).toBeInTheDocument()
  })

  it('shows serial when available', () => {
    mockUseListKoboDevices.mockReturnValue(makeUseListKoboDevices([device]))
    render(<KoboDevices />)
    expect(screen.getByText(/N4181234/)).toBeInTheDocument()
  })

  it('shows "Never synced" when lastSeenAt is empty', () => {
    mockUseListKoboDevices.mockReturnValue(makeUseListKoboDevices([deviceNoLastSeen]))
    render(<KoboDevices />)
    expect(screen.getByText(/Never synced/)).toBeInTheDocument()
  })

  it('shows last synced date when lastSeenAt is set', () => {
    mockUseListKoboDevices.mockReturnValue(makeUseListKoboDevices([device]))
    render(<KoboDevices />)
    expect(screen.getByText(/Last synced/)).toBeInTheDocument()
  })

  it('renders a Disconnect button per device', () => {
    mockUseListKoboDevices.mockReturnValue(makeUseListKoboDevices([device]))
    render(<KoboDevices />)
    expect(screen.getByTestId(`kobo-disconnect-btn-${device.id}`)).toBeInTheDocument()
  })
})

describe('KoboDevices — disconnect flow', () => {
  const device = {
    id: 'dev-abc',
    name: 'My Kobo',
    serial: '',
    createdAt: '2024-01-01T00:00:00Z',
    lastSeenAt: ''
  }

  beforeEach(() => {
    mockUseListKoboDevices.mockReturnValue(makeUseListKoboDevices([device]))
  })

  it('opens confirmation dialog when Disconnect is clicked', () => {
    render(<KoboDevices />)
    fireEvent.click(screen.getByTestId(`kobo-disconnect-btn-${device.id}`))
    expect(screen.getByTestId('dialog')).toBeInTheDocument()
    expect(screen.getByText(/Disconnect My Kobo/)).toBeInTheDocument()
  })

  it('calls disconnectKoboDevice and mutates on confirm', async () => {
    render(<KoboDevices />)
    fireEvent.click(screen.getByTestId(`kobo-disconnect-btn-${device.id}`))

    await act(async () => {
      fireEvent.click(screen.getByTestId('disconnect-confirm-btn'))
    })

    await waitFor(() => {
      expect(mockDisconnectKoboDevice).toHaveBeenCalledWith(device.id)
    })
    expect(mockMutate).toHaveBeenCalled()
  })

  it('closes dialog when disconnect succeeds', async () => {
    render(<KoboDevices />)
    fireEvent.click(screen.getByTestId(`kobo-disconnect-btn-${device.id}`))

    await act(async () => {
      fireEvent.click(screen.getByTestId('disconnect-confirm-btn'))
    })

    await waitFor(() => {
      expect(screen.queryByTestId('dialog')).not.toBeInTheDocument()
    })
  })

  it('shows error message when disconnect fails', async () => {
    mockDisconnectKoboDevice.mockRejectedValueOnce(new Error('Server error'))
    render(<KoboDevices />)
    fireEvent.click(screen.getByTestId(`kobo-disconnect-btn-${device.id}`))

    await act(async () => {
      fireEvent.click(screen.getByTestId('disconnect-confirm-btn'))
    })

    await waitFor(() => {
      expect(screen.getByTestId('disconnect-error')).toBeInTheDocument()
    })
    expect(mockMutate).not.toHaveBeenCalled()
  })
})

describe('KoboDevices — debug logging', () => {
  const offDevice = {
    id: 'dev-off',
    name: 'Logging Off Kobo',
    serial: '',
    createdAt: '2024-01-01T00:00:00Z',
    lastSeenAt: '',
    loggingEnabled: false
  }
  const onDevice = {
    id: 'dev-on',
    name: 'Logging On Kobo',
    serial: '',
    createdAt: '2024-01-01T00:00:00Z',
    lastSeenAt: '',
    loggingEnabled: true
  }

  it('renders a debug-logging toggle per device', () => {
    mockUseListKoboDevices.mockReturnValue(makeUseListKoboDevices([offDevice]))
    render(<KoboDevices />)
    const toggle = screen.getByTestId(`kobo-logging-toggle-${offDevice.id}`)
    expect(toggle).not.toBeChecked()
  })

  it('enables logging and mutates when toggled on', async () => {
    mockUseListKoboDevices.mockReturnValue(makeUseListKoboDevices([offDevice]))
    render(<KoboDevices />)

    await act(async () => {
      fireEvent.click(screen.getByTestId(`kobo-logging-toggle-${offDevice.id}`))
    })

    await waitFor(() => {
      expect(mockSetKoboDeviceLogging).toHaveBeenCalledWith(offDevice.id, true)
    })
    expect(mockMutate).toHaveBeenCalled()
  })

  it('shows View logs and expands the viewer for a logging-enabled device', async () => {
    mockUseListKoboDevices.mockReturnValue(makeUseListKoboDevices([onDevice]))
    render(<KoboDevices />)

    const toggleBtn = screen.getByTestId(`kobo-logs-toggle-${onDevice.id}`)
    expect(toggleBtn).toHaveTextContent('View logs')
    expect(screen.queryByTestId(`kobo-logs-stub-${onDevice.id}`)).not.toBeInTheDocument()

    await act(async () => {
      fireEvent.click(toggleBtn)
    })

    expect(screen.getByTestId(`kobo-logs-stub-${onDevice.id}`)).toBeInTheDocument()
    expect(screen.getByTestId(`kobo-logs-toggle-${onDevice.id}`)).toHaveTextContent('Hide logs')
  })

  it('does not show View logs when logging is disabled', () => {
    mockUseListKoboDevices.mockReturnValue(makeUseListKoboDevices([offDevice]))
    render(<KoboDevices />)
    expect(screen.queryByTestId(`kobo-logs-toggle-${offDevice.id}`)).not.toBeInTheDocument()
  })
})
