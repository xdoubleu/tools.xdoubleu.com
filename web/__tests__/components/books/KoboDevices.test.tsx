import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'

const mockDisconnectKoboDevice = jest.fn()
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
  useDisconnectKoboDevice: () => mockDisconnectKoboDevice
}))

// Stub Dialog to render children inline (avoids portal issues in jsdom).
jest.mock('@/components/ui/dialog', () => ({
  Dialog: ({ children, open }: { children: React.ReactNode; open: boolean }) =>
    open ? <div data-testid="dialog">{children}</div> : null,
  DialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>
}))

import KoboDevices from '@/components/books/KoboDevices'

beforeEach(() => {
  mockDisconnectKoboDevice.mockReset()
  mockMutate.mockReset()
  mockDisconnectKoboDevice.mockResolvedValue({})
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
