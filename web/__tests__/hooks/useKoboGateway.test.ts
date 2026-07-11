import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/books/gatewayClient', () => ({
  probeGateway: jest.fn()
}))

import useSWR from 'swr'
import { useGatewayStatus } from '@/hooks/useKoboGateway'
import { swrKeys } from '@/lib/swrKeys'

const mockUseSWR = jest.mocked(useSWR)

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
})

describe('useGatewayStatus', () => {
  it('polls the gateway status key on a fixed interval', () => {
    renderHook(() => useGatewayStatus())

    expect(mockUseSWR).toHaveBeenCalledWith(
      swrKeys.gatewayStatus,
      expect.any(Function),
      expect.objectContaining({ refreshInterval: 2000 })
    )
  })
})
