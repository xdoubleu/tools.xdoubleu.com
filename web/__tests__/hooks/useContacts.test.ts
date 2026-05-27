import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    listContacts: jest.fn(),
    createContact: jest.fn(),
    acceptContact: jest.fn(),
    declineContact: jest.fn(),
    deleteContact: jest.fn()
  }))
}))
jest.mock('@/lib/gen/contacts/v1/contacts_pb', () => ({
  ContactsService: {}
}))

import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import {
  useContacts,
  useCreateContact,
  useAcceptContact,
  useDeclineContact,
  useDeleteContact
} from '@/hooks/useContacts'

const mockUseSWR = useSWR as jest.Mock
const mockCreateServiceClient = createServiceClient as jest.Mock

beforeEach(() => {
  mockUseSWR.mockReturnValue({
    data: undefined,
    isLoading: false,
    error: undefined
  })
  mockUseSWR.mockClear()
})

describe('useContacts', () => {
  it('uses /contacts as key', () => {
    renderHook(() => useContacts())
    expect(mockUseSWR).toHaveBeenCalledWith('/contacts', expect.any(Function))
  })

  it('returns SWR result', () => {
    const mockData = { contacts: [], pending: [], incoming: [] }
    mockUseSWR.mockReturnValueOnce({
      data: mockData,
      isLoading: false,
      error: undefined
    })
    const { result } = renderHook(() => useContacts())
    expect(result.current.data).toEqual(mockData)
  })
})

describe('useCreateContact', () => {
  it('returns a function that calls client.createContact', () => {
    const mockCreateContact = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      createContact: mockCreateContact
    })

    const { result } = renderHook(() => useCreateContact())
    result.current('a@b.com', 'Alice')
    expect(mockCreateContact).toHaveBeenCalledWith({
      email: 'a@b.com',
      displayName: 'Alice'
    })
  })
})

describe('useAcceptContact', () => {
  it('returns a function that calls client.acceptContact', () => {
    const mockAcceptContact = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      acceptContact: mockAcceptContact
    })

    const { result } = renderHook(() => useAcceptContact())
    result.current('contact-1', 'Bob')
    expect(mockAcceptContact).toHaveBeenCalledWith({
      id: 'contact-1',
      displayName: 'Bob'
    })
  })
})

describe('useDeclineContact', () => {
  it('returns a function that calls client.declineContact', () => {
    const mockDeclineContact = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      declineContact: mockDeclineContact
    })

    const { result } = renderHook(() => useDeclineContact())
    result.current('contact-2')
    expect(mockDeclineContact).toHaveBeenCalledWith({ id: 'contact-2' })
  })
})

describe('useDeleteContact', () => {
  it('returns a function that calls client.deleteContact', () => {
    const mockDeleteContact = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      deleteContact: mockDeleteContact
    })

    const { result } = renderHook(() => useDeleteContact())
    result.current('contact-3')
    expect(mockDeleteContact).toHaveBeenCalledWith({ id: 'contact-3' })
  })
})
