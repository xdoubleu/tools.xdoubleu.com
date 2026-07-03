import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'

jest.mock('@/lib/env', () => ({
  getApiUrl: () => 'https://api.example.com'
}))

jest.mock('@/lib/books/koboConf', () => {
  const actual = jest.requireActual('@/lib/books/koboConf')
  return actual
})

// Mock koboDevice helpers so tests don't need a real .kobo/version file.
jest.mock('@/lib/books/koboDevice', () => ({
  readKoboSerial: jest.fn().mockResolvedValue('N418ABCD1234'),
  defaultDeviceName: jest.fn().mockReturnValue('Kobo (…1234)')
}))

const mockRegisterKoboDevice = jest.fn()
const mockDisconnectKoboDevice = jest.fn()
const mockMutateDevices = jest.fn()

// Allow per-test override of the devices list returned by useListKoboDevices.
let mockDevicesData: { devices: { id: string; serial: string; name: string }[] } | undefined =
  undefined

jest.mock('@/hooks/useBooks', () => ({
  useRegisterKoboDevice: () => mockRegisterKoboDevice,
  useDisconnectKoboDevice: () => mockDisconnectKoboDevice,
  useListKoboDevices: () => ({ data: mockDevicesData, mutate: mockMutateDevices })
}))

// --- FS helper factories ---

const SAMPLE_CONF_UNMANAGED = `[OneStoreServices]
api_endpoint=https://storeapi.kobo.com
affiliate=Kobo`

const SAMPLE_CONF_MANAGED = `[OneStoreServices]
api_endpoint=https://api.example.com/books/kobo/some-token
affiliate=Kobo`

function makeWritable() {
  return {
    write: jest.fn().mockResolvedValue(undefined),
    close: jest.fn().mockResolvedValue(undefined)
  }
}

function makeFileHandle(content: string) {
  const writable = makeWritable()
  const mockFile = { text: jest.fn().mockResolvedValue(content) }
  return {
    getFile: jest.fn().mockResolvedValue(mockFile),
    createWritable: jest.fn().mockResolvedValue(writable),
    _writable: writable
  }
}

function makeKoboRoot(confContent: string) {
  const confHandle = makeFileHandle(confContent)
  const innerDir = { getFileHandle: jest.fn().mockResolvedValue(confHandle) }
  const koboDir = { getDirectoryHandle: jest.fn().mockResolvedValue(innerDir) }
  const rootDir = { getDirectoryHandle: jest.fn().mockResolvedValue(koboDir) }
  return { rootDir, confHandle }
}

function defineShowDirectoryPicker(mock: jest.Mock) {
  Object.defineProperty(window, 'showDirectoryPicker', {
    value: mock,
    writable: true,
    configurable: true
  })
}

function removeShowDirectoryPicker() {
  Object.defineProperty(window, 'showDirectoryPicker', {
    value: undefined,
    writable: true,
    configurable: true
  })
}

import KoboSetup from '@/components/books/KoboSetup'

beforeEach(() => {
  mockRegisterKoboDevice.mockReset()
  mockDisconnectKoboDevice.mockReset()
  mockMutateDevices.mockReset()
  mockDevicesData = undefined
  mockRegisterKoboDevice.mockResolvedValue({ device: { id: 'dev-1' }, rawToken: 'my-token' })
  mockDisconnectKoboDevice.mockResolvedValue({})
})

describe('KoboSetup — unsupported browser', () => {
  beforeEach(() => removeShowDirectoryPicker())

  it('renders fallback when FS API is not supported', () => {
    render(<KoboSetup />)
    expect(screen.getByTestId('kobo-setup-fallback')).toBeInTheDocument()
  })

  it('does not render the detect button in fallback', () => {
    render(<KoboSetup />)
    expect(screen.queryByTestId('kobo-detect-btn')).not.toBeInTheDocument()
  })
})

describe('KoboSetup — supported browser, idle state', () => {
  let mockPicker: jest.Mock

  beforeEach(() => {
    mockPicker = jest.fn()
    defineShowDirectoryPicker(mockPicker)
  })

  it('renders the Select button in idle state', () => {
    mockPicker.mockRejectedValue(Object.assign(new Error('abort'), { name: 'AbortError' }))
    render(<KoboSetup />)
    expect(screen.getByTestId('kobo-detect-btn')).toBeInTheDocument()
    expect(screen.getByTestId('kobo-detect-btn')).toHaveTextContent('Select my Kobo')
  })

  it('returns to idle when user cancels the directory picker', async () => {
    const abortErr = Object.assign(new Error('abort'), { name: 'AbortError' })
    mockPicker.mockRejectedValue(abortErr)

    render(<KoboSetup />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-detect-btn'))
    })

    await waitFor(() => {
      expect(screen.getByTestId('kobo-detect-btn')).toHaveTextContent('Select my Kobo')
    })
    expect(screen.queryByTestId('kobo-setup-error')).not.toBeInTheDocument()
  })

  it('shows error when root is not a Kobo drive', async () => {
    const notKoboDir = {
      getDirectoryHandle: jest.fn().mockRejectedValue(new Error('Directory not found'))
    }
    mockPicker.mockResolvedValue(notKoboDir)

    render(<KoboSetup />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-detect-btn'))
    })

    await waitFor(() => {
      expect(screen.getByTestId('kobo-setup-error')).toBeInTheDocument()
    })
  })
})

describe('KoboSetup — supported browser, device not configured', () => {
  let mockPicker: jest.Mock

  beforeEach(() => {
    mockPicker = jest.fn()
    defineShowDirectoryPicker(mockPicker)
  })

  it('registers device and shows success after configure', async () => {
    const { rootDir, confHandle } = makeKoboRoot(SAMPLE_CONF_UNMANAGED)
    mockPicker.mockResolvedValue(rootDir)

    render(<KoboSetup />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-detect-btn'))
    })

    await waitFor(() => {
      expect(screen.getByTestId('kobo-setup-success')).toBeInTheDocument()
    })

    expect(mockRegisterKoboDevice).toHaveBeenCalledWith('Kobo (…1234)', 'N418ABCD1234')
    expect(confHandle.createWritable).toHaveBeenCalled()
    expect(confHandle._writable.write).toHaveBeenCalledWith(
      expect.stringContaining('api_endpoint=https://api.example.com/books/kobo/my-token')
    )
    expect(confHandle._writable.close).toHaveBeenCalled()
    expect(mockMutateDevices).toHaveBeenCalled()
  })

  it('preserves other conf keys after patching', async () => {
    const { rootDir, confHandle } = makeKoboRoot(SAMPLE_CONF_UNMANAGED)
    mockPicker.mockResolvedValue(rootDir)

    render(<KoboSetup />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-detect-btn'))
    })

    await waitFor(() => screen.getByTestId('kobo-setup-success'))

    expect(confHandle._writable.write).toHaveBeenCalledWith(
      expect.stringContaining('affiliate=Kobo')
    )
  })

  it('shows Revert button after successful configure', async () => {
    const { rootDir } = makeKoboRoot(SAMPLE_CONF_UNMANAGED)
    mockPicker.mockResolvedValue(rootDir)

    render(<KoboSetup />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-detect-btn'))
    })

    await waitFor(() => screen.getByTestId('kobo-revert-btn'))
    expect(screen.getByTestId('kobo-revert-btn')).toHaveTextContent('Revert configuration')
  })

  it('reverts conf and disconnects device after clicking Revert', async () => {
    const { rootDir, confHandle } = makeKoboRoot(SAMPLE_CONF_UNMANAGED)
    mockPicker.mockResolvedValue(rootDir)

    render(<KoboSetup />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-detect-btn'))
    })
    await waitFor(() => screen.getByTestId('kobo-revert-btn'))

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-revert-btn'))
    })

    await waitFor(() => {
      expect(screen.getByTestId('kobo-detect-btn')).toBeInTheDocument()
    })

    // Conf must be reverted to original endpoint.
    const calls = (confHandle._writable.write as jest.Mock).mock.calls
    expect(String(calls[1][0])).toContain('api_endpoint=https://storeapi.kobo.com')

    // Device token must be revoked.
    expect(mockDisconnectKoboDevice).toHaveBeenCalledWith('dev-1')
    expect(mockMutateDevices).toHaveBeenCalledTimes(2) // once on configure, once on revert
  })

  it('shows error when revert conf read fails', async () => {
    const { rootDir, confHandle } = makeKoboRoot(SAMPLE_CONF_UNMANAGED)
    mockPicker.mockResolvedValue(rootDir)

    render(<KoboSetup />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-detect-btn'))
    })
    await waitFor(() => screen.getByTestId('kobo-revert-btn'))

    confHandle.getFile.mockRejectedValue(new Error('Read failed during revert'))

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-revert-btn'))
    })

    await waitFor(() => {
      expect(screen.getByTestId('kobo-setup-error')).toHaveTextContent('Read failed during revert')
    })
  })
})

describe('KoboSetup — supported browser, device already configured', () => {
  let mockPicker: jest.Mock

  beforeEach(() => {
    mockPicker = jest.fn()
    defineShowDirectoryPicker(mockPicker)
  })

  it('shows already-configured banner when endpoint points to this server', async () => {
    const { rootDir } = makeKoboRoot(SAMPLE_CONF_MANAGED)
    mockPicker.mockResolvedValue(rootDir)

    render(<KoboSetup />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-detect-btn'))
    })

    await waitFor(() => {
      expect(screen.getByTestId('kobo-already-configured')).toBeInTheDocument()
    })

    // Should NOT have called registerKoboDevice — device is already set up.
    expect(mockRegisterKoboDevice).not.toHaveBeenCalled()
  })

  it('shows revert button in already-configured state', async () => {
    const { rootDir } = makeKoboRoot(SAMPLE_CONF_MANAGED)
    mockPicker.mockResolvedValue(rootDir)

    render(<KoboSetup />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-detect-btn'))
    })

    await waitFor(() => screen.getByTestId('kobo-already-configured'))
    expect(screen.getByTestId('kobo-revert-btn')).toHaveTextContent(
      'Revert to original Kobo settings'
    )
  })

  it('reverts conf to stock Kobo endpoint and disconnects matching device', async () => {
    // Serial returned by readKoboSerial mock is 'N418ABCD1234'.
    mockDevicesData = {
      devices: [{ id: 'dev-managed', serial: 'N418ABCD1234', name: 'Kobo (…1234)' }]
    }

    const { rootDir, confHandle } = makeKoboRoot(SAMPLE_CONF_MANAGED)
    mockPicker.mockResolvedValue(rootDir)

    render(<KoboSetup />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-detect-btn'))
    })
    await waitFor(() => screen.getByTestId('kobo-revert-btn'))

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-revert-btn'))
    })

    await waitFor(() => {
      expect(screen.getByTestId('kobo-detect-btn')).toBeInTheDocument()
    })

    // Conf must be written back with the stock Kobo endpoint.
    expect(confHandle._writable.write).toHaveBeenCalledWith(
      expect.stringContaining('api_endpoint=https://storeapi.kobo.com')
    )

    // Device token must be revoked by the serial-matched ID.
    expect(mockDisconnectKoboDevice).toHaveBeenCalledWith('dev-managed')
    expect(mockMutateDevices).toHaveBeenCalled()
  })

  it('still reverts conf when no device matches the serial (orphaned config)', async () => {
    // Device list has a different serial — no match.
    mockDevicesData = { devices: [{ id: 'dev-other', serial: 'XXXXXXXXXX', name: 'Other Kobo' }] }

    const { rootDir, confHandle } = makeKoboRoot(SAMPLE_CONF_MANAGED)
    mockPicker.mockResolvedValue(rootDir)

    render(<KoboSetup />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-detect-btn'))
    })
    await waitFor(() => screen.getByTestId('kobo-revert-btn'))

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-revert-btn'))
    })

    await waitFor(() => {
      expect(screen.getByTestId('kobo-detect-btn')).toBeInTheDocument()
    })

    // Conf is still reverted even though no device matched.
    expect(confHandle._writable.write).toHaveBeenCalledWith(
      expect.stringContaining('api_endpoint=https://storeapi.kobo.com')
    )

    // No disconnect call when serial doesn't match.
    expect(mockDisconnectKoboDevice).not.toHaveBeenCalled()
  })
})
