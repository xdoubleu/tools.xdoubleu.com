import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    listConfigs: jest.fn(),
    previewEvents: jest.fn(),
    getConfig: jest.fn(),
    saveConfig: jest.fn(),
    deleteConfig: jest.fn()
  }))
}))
jest.mock('@/lib/gen/icsproxy/v1/proxy_pb', () => ({
  ICSProxyService: {},
  SaveConfigRequestSchema: {}
}))

import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import {
  useICSFeeds,
  useICSPreview,
  useICSConfig,
  useSaveConfig,
  useDeleteConfig
} from '@/hooks/useICSProxy'

const mockUseSWR = useSWR as jest.Mock
const mockCreateServiceClient = createServiceClient as jest.Mock

beforeEach(() => {
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
})

describe('useICSFeeds', () => {
  it('uses /icsproxy as key', () => {
    renderHook(() => useICSFeeds())
    expect(mockUseSWR).toHaveBeenCalledWith('/icsproxy', expect.any(Function))
  })

  it('fetcher calls client.listConfigs', async () => {
    const mockListConfigs = jest.fn().mockResolvedValue({ configs: [] })
    mockCreateServiceClient.mockReturnValue({ listConfigs: mockListConfigs })

    renderHook(() => useICSFeeds())
    const fetcher = mockUseSWR.mock.calls[0][1]
    await fetcher()
    expect(mockListConfigs).toHaveBeenCalledWith({})
  })

  it('returns SWR result', () => {
    const mockData = { configs: [{ token: 't1' }] }
    mockUseSWR.mockReturnValueOnce({ data: mockData, isLoading: false, error: undefined })
    const { result } = renderHook(() => useICSFeeds())
    expect(result.current.data).toEqual(mockData)
  })
})

describe('useICSPreview', () => {
  it('encodes the sourceUrl in the key when given', () => {
    renderHook(() => useICSPreview('https://cal.example.com/feed.ics'))
    const [key] = mockUseSWR.mock.calls[0]
    expect(key).toContain('/icsproxy/preview?url=')
    expect(key).toContain('cal.example.com')
  })

  it('passes null as key when sourceUrl is empty', () => {
    renderHook(() => useICSPreview(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })

  it('fetcher calls client.previewEvents with sourceUrl', async () => {
    const mockPreviewEvents = jest.fn().mockResolvedValue({ events: [] })
    mockCreateServiceClient.mockReturnValue({ previewEvents: mockPreviewEvents })

    const url = 'https://cal.example.com/feed.ics'
    renderHook(() => useICSPreview(url))
    const fetcher = mockUseSWR.mock.calls[0][1]
    await fetcher()
    expect(mockPreviewEvents).toHaveBeenCalledWith({ sourceUrl: url })
  })
})

describe('useICSConfig', () => {
  it('uses correct key when token is provided', () => {
    renderHook(() => useICSConfig('tok-1'))
    expect(mockUseSWR).toHaveBeenCalledWith('/icsproxy/tok-1', expect.any(Function))
  })

  it('passes null as key when token is empty', () => {
    renderHook(() => useICSConfig(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })

  it('fetcher calls client.getConfig with token', async () => {
    const mockGetConfig = jest.fn().mockResolvedValue({ config: {} })
    mockCreateServiceClient.mockReturnValue({ getConfig: mockGetConfig })

    renderHook(() => useICSConfig('tok-1'))
    const fetcher = mockUseSWR.mock.calls[0][1]
    await fetcher()
    expect(mockGetConfig).toHaveBeenCalledWith({ token: 'tok-1' })
  })
})

describe('useSaveConfig', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useSaveConfig())
    expect(typeof result.current).toBe('function')
  })

  it('calls client.saveConfig with the provided request', () => {
    const mockSaveConfig = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({ saveConfig: mockSaveConfig })

    const { result } = renderHook(() => useSaveConfig())
    const req = { sourceUrl: 'https://cal.example.com/feed.ics' }
    result.current(req)
    expect(mockSaveConfig).toHaveBeenCalledWith(req)
  })
})

describe('useDeleteConfig', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useDeleteConfig())
    expect(typeof result.current).toBe('function')
  })

  it('calls client.deleteConfig with the provided token', () => {
    const mockDeleteConfig = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({ deleteConfig: mockDeleteConfig })

    const { result } = renderHook(() => useDeleteConfig())
    result.current('tok-1')
    expect(mockDeleteConfig).toHaveBeenCalledWith({ token: 'tok-1' })
  })
})
