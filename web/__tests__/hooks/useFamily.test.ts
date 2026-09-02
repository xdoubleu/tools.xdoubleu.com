import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    getFamily: jest.fn(),
    inviteToFamily: jest.fn(),
    acceptFamilyInvite: jest.fn(),
    declineFamilyInvite: jest.fn(),
    leaveFamily: jest.fn()
  }))
}))
jest.mock('@/lib/gen/family/v1/family_pb', () => ({
  FamilyService: {}
}))

import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import {
  useFamily,
  useInviteToFamily,
  useAcceptFamilyInvite,
  useDeclineFamilyInvite,
  useLeaveFamily
} from '@/hooks/useFamily'

const mockUseSWR = jest.mocked(useSWR)
const mockCreateServiceClient = jest.mocked(createServiceClient)

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockReturnValue({
    data: undefined,
    isLoading: false,
    error: undefined
  })
  mockUseSWR.mockClear()
})

describe('useFamily', () => {
  it('uses /family as key', () => {
    renderHook(() => useFamily())
    expect(mockUseSWR).toHaveBeenCalledWith('/family', expect.any(Function))
  })

  it('returns SWR result', () => {
    const mockData = { members: [], incomingInvite: undefined }
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSWR.mockReturnValueOnce({
      data: mockData,
      isLoading: false,
      error: undefined
    })
    const { result } = renderHook(() => useFamily())
    expect(result.current.data).toEqual(mockData)
  })
})

describe('useInviteToFamily', () => {
  it('returns a function that calls client.inviteToFamily', () => {
    const mockInvite = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      inviteToFamily: mockInvite
    })

    const { result } = renderHook(() => useInviteToFamily())
    result.current('a@b.com')
    expect(mockInvite).toHaveBeenCalledWith({ email: 'a@b.com' })
  })
})

describe('useAcceptFamilyInvite', () => {
  it('returns a function that calls client.acceptFamilyInvite', () => {
    const mockAccept = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      acceptFamilyInvite: mockAccept
    })

    const { result } = renderHook(() => useAcceptFamilyInvite())
    result.current()
    expect(mockAccept).toHaveBeenCalledWith({})
  })
})

describe('useDeclineFamilyInvite', () => {
  it('returns a function that calls client.declineFamilyInvite', () => {
    const mockDecline = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      declineFamilyInvite: mockDecline
    })

    const { result } = renderHook(() => useDeclineFamilyInvite())
    result.current()
    expect(mockDecline).toHaveBeenCalledWith({})
  })
})

describe('useLeaveFamily', () => {
  it('returns a function that calls client.leaveFamily', () => {
    const mockLeave = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      leaveFamily: mockLeave
    })

    const { result } = renderHook(() => useLeaveFamily())
    result.current()
    expect(mockLeave).toHaveBeenCalledWith({})
  })
})
