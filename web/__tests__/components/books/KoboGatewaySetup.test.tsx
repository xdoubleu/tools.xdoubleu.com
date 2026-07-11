import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'

jest.mock('@/lib/env', () => ({
  getApiUrl: () => 'https://api.example.com'
}))

jest.mock('@/lib/books/koboDevice', () => ({
  defaultDeviceName: (serial: string) =>
    serial.length >= 4 ? `Kobo (…${serial.slice(-4)})` : 'My Kobo'
}))

const mockConfigureGateway = jest.fn()
const mockRevertGateway = jest.fn()
const mockUpdateGateway = jest.fn()

jest.mock('@/lib/books/gatewayClient', () => ({
  REQUIRED_GATEWAY_VERSION: 1,
  configureGateway: (...args: unknown[]) => mockConfigureGateway(...args),
  revertGateway: (...args: unknown[]) => mockRevertGateway(...args),
  updateGateway: (...args: unknown[]) => mockUpdateGateway(...args)
}))

// The polling hook is driven manually in these tests via mockMutateGatewayStatus,
// standing in for what SWR's mutate() would return from a fresh /status probe.
const mockMutateGatewayStatus = jest.fn()
jest.mock('@/hooks/useKoboGateway', () => ({
  useGatewayStatus: () => ({ mutate: mockMutateGatewayStatus })
}))

const mockRegisterKoboDevice = jest.fn()
const mockDisconnectKoboDevice = jest.fn()
const mockMutateDevices = jest.fn()
let mockDevicesData: { devices: { id: string; serial: string; name: string }[] } | undefined =
  undefined

jest.mock('@/hooks/useBooks', () => ({
  useRegisterKoboDevice: () => mockRegisterKoboDevice,
  useDisconnectKoboDevice: () => mockDisconnectKoboDevice,
  useListKoboDevices: () => ({ data: mockDevicesData, mutate: mockMutateDevices })
}))

import KoboGatewaySetup from '@/components/books/KoboGatewaySetup'

const KOBO_UNMANAGED = {
  volumePath: '/Volumes/KOBOeReader',
  serial: 'N418ABCD1234',
  currentEndpoint: 'https://storeapi.kobo.com'
}

const KOBO_MANAGED = {
  ...KOBO_UNMANAGED,
  currentEndpoint: 'https://api.example.com/books/kobo/some-token'
}

function status(kobos: (typeof KOBO_UNMANAGED)[], version = 1) {
  return { version, release: 'abc1234', kobos }
}

beforeEach(() => {
  mockMutateGatewayStatus.mockReset()
  mockConfigureGateway.mockReset()
  mockRevertGateway.mockReset()
  mockUpdateGateway.mockReset()
  mockRegisterKoboDevice.mockReset()
  mockDisconnectKoboDevice.mockReset()
  mockMutateDevices.mockReset()
  mockDevicesData = undefined

  mockRegisterKoboDevice.mockResolvedValue({ device: { id: 'dev-1' }, rawToken: 'my-token' })
  mockDisconnectKoboDevice.mockResolvedValue({})
  mockConfigureGateway.mockResolvedValue({
    serial: 'N418ABCD1234',
    originalEndpoint: 'https://storeapi.kobo.com'
  })
  mockRevertGateway.mockResolvedValue({ serial: 'N418ABCD1234' })
  mockUpdateGateway.mockResolvedValue({ updating: true })
  mockMutateGatewayStatus.mockResolvedValue(status([]))
})

describe('KoboGatewaySetup — no Kobo connected', () => {
  it('shows a message telling the user to plug in a Kobo', () => {
    render(<KoboGatewaySetup status={status([])} />)

    expect(screen.getByTestId('kobo-gateway-no-kobo')).toBeInTheDocument()
  })
})

describe('KoboGatewaySetup — fresh device', () => {
  it('registers the device and configures via the gateway', async () => {
    render(<KoboGatewaySetup status={status([KOBO_UNMANAGED])} />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-gateway-configure-btn'))
    })

    await waitFor(() => {
      expect(screen.getByTestId('kobo-gateway-success')).toBeInTheDocument()
    })

    expect(mockRegisterKoboDevice).toHaveBeenCalledWith('Kobo (…1234)', 'N418ABCD1234')
    expect(mockConfigureGateway).toHaveBeenCalledWith(
      'https://api.example.com/books/kobo/my-token',
      '/Volumes/KOBOeReader'
    )
    expect(mockMutateDevices).toHaveBeenCalled()
  })

  it('reverts to the original endpoint and revokes the new device', async () => {
    render(<KoboGatewaySetup status={status([KOBO_UNMANAGED])} />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-gateway-configure-btn'))
    })
    await waitFor(() => screen.getByTestId('kobo-gateway-revert-btn'))

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-gateway-revert-btn'))
    })

    await waitFor(() => {
      expect(screen.getByTestId('kobo-gateway-configure-btn')).toBeInTheDocument()
    })

    expect(mockRevertGateway).toHaveBeenCalledWith(
      'https://storeapi.kobo.com',
      '/Volumes/KOBOeReader'
    )
    expect(mockDisconnectKoboDevice).toHaveBeenCalledWith('dev-1')
  })

  it('shows the gateway error and can be dismissed when configuring fails', async () => {
    mockConfigureGateway.mockRejectedValue(new Error('could not write Kobo eReader.conf'))

    render(<KoboGatewaySetup status={status([KOBO_UNMANAGED])} />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-gateway-configure-btn'))
    })

    await waitFor(() => {
      expect(screen.getByTestId('kobo-gateway-error')).toHaveTextContent(
        'could not write Kobo eReader.conf'
      )
    })

    fireEvent.click(screen.getByText('Dismiss'))
    expect(screen.queryByTestId('kobo-gateway-error')).not.toBeInTheDocument()
  })
})

describe('KoboGatewaySetup — already configured', () => {
  it('shows the banner and reverts to stock settings, matching device by serial', async () => {
    mockDevicesData = {
      devices: [{ id: 'dev-managed', serial: 'N418ABCD1234', name: 'Kobo (…1234)' }]
    }

    render(<KoboGatewaySetup status={status([KOBO_MANAGED])} />)

    expect(screen.getByTestId('kobo-gateway-already-configured')).toBeInTheDocument()

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-gateway-revert-btn'))
    })

    await waitFor(() => {
      expect(mockRevertGateway).toHaveBeenCalledWith(
        'https://storeapi.kobo.com',
        '/Volumes/KOBOeReader'
      )
    })
    expect(mockDisconnectKoboDevice).toHaveBeenCalledWith('dev-managed')
    expect(mockRegisterKoboDevice).not.toHaveBeenCalled()
  })

  it('still reverts when no device matches the serial', async () => {
    mockDevicesData = { devices: [{ id: 'dev-other', serial: 'XXXX', name: 'Other' }] }

    render(<KoboGatewaySetup status={status([KOBO_MANAGED])} />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-gateway-revert-btn'))
    })

    await waitFor(() => {
      expect(mockRevertGateway).toHaveBeenCalled()
    })
    expect(mockDisconnectKoboDevice).not.toHaveBeenCalled()
  })
})

describe('KoboGatewaySetup — multiple Kobos', () => {
  it('offers a picker and configures the chosen volume', async () => {
    const second = { ...KOBO_UNMANAGED, volumePath: '/Volumes/KOBO2', serial: 'ZZ99' }

    render(<KoboGatewaySetup status={status([KOBO_UNMANAGED, second])} />)

    expect(screen.getByTestId('kobo-gateway-picker')).toBeInTheDocument()

    fireEvent.click(screen.getByText(/\/Volumes\/KOBO2/))

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-gateway-configure-btn'))
    })

    await waitFor(() => screen.getByTestId('kobo-gateway-success'))
    expect(mockConfigureGateway).toHaveBeenCalledWith(
      'https://api.example.com/books/kobo/my-token',
      '/Volumes/KOBO2'
    )
  })
})

describe('KoboGatewaySetup — self-update', () => {
  it('updates an outdated gateway and continues once it is back', async () => {
    mockMutateGatewayStatus.mockResolvedValue(status([KOBO_UNMANAGED]))

    render(<KoboGatewaySetup status={status([KOBO_UNMANAGED], 0)} pollIntervalMs={1} />)

    expect(screen.getByTestId('kobo-gateway-updating')).toBeInTheDocument()

    await waitFor(() => {
      expect(mockUpdateGateway).toHaveBeenCalled()
    })
    await waitFor(() => {
      expect(screen.queryByTestId('kobo-gateway-updating')).not.toBeInTheDocument()
    })
  })

  it('shows manual instructions when the update fails', async () => {
    mockUpdateGateway.mockRejectedValue(new Error('update download failed'))

    render(<KoboGatewaySetup status={status([KOBO_UNMANAGED], 0)} pollIntervalMs={1} />)

    await waitFor(() => {
      expect(screen.getByTestId('kobo-gateway-error')).toHaveTextContent('update download failed')
    })
    expect(screen.getByTestId('kobo-gateway-error')).toHaveTextContent(
      'Download the latest version manually'
    )
  })

  it('errors when the gateway never comes back after updating', async () => {
    mockMutateGatewayStatus.mockResolvedValue(null)

    render(<KoboGatewaySetup status={status([KOBO_UNMANAGED], 0)} pollIntervalMs={1} />)

    await waitFor(() => {
      expect(screen.getByTestId('kobo-gateway-error')).toHaveTextContent(
        'did not come back after updating'
      )
    })
  })
})
