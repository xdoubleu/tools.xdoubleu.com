import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
const mockGetPhysicalCopy = jest.fn()
jest.mock('@/lib/client', () => ({
  createServiceClient: () => ({ getPhysicalCopy: mockGetPhysicalCopy })
}))
jest.mock('@/lib/gen/books/v1/library_pb', () => ({ LibraryService: {} }))
const mockEnqueueWrite = jest.fn()
const mockSendWrite = jest.fn()
jest.mock('@/lib/offline/outbox', () => ({
  enqueueWrite: (...args: unknown[]) => mockEnqueueWrite(...args),
  sendWrite: (...args: unknown[]) => mockSendWrite(...args)
}))
jest.mock('@/lib/books/offlineWrites', () => ({ updatePhysicalCopyWrite: 'write' }))

import useSWR from 'swr'
import { usePhysicalCopy, useUpdatePhysicalCopy } from '@/hooks/usePhysicalCopy'
import { swrKeys } from '@/lib/swrKeys'

const mockUseSWR = jest.mocked(useSWR)

describe('usePhysicalCopy', () => {
  beforeEach(() => mockUseSWR.mockReset())

  it('fetches the copy details', async () => {
    renderHook(() => usePhysicalCopy('book-1'))
    expect(mockUseSWR.mock.calls[0]![0]).toEqual(swrKeys.physicalCopy('book-1'))
    mockGetPhysicalCopy.mockResolvedValue({ syncable: true })
    await expect(mockUseSWR.mock.calls[0]![1]!()).resolves.toEqual({ syncable: true })
    expect(mockGetPhysicalCopy).toHaveBeenCalledWith({ bookId: 'book-1' })
  })

  it('waits for a book id', () => {
    renderHook(() => usePhysicalCopy(null))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })
})

describe('useUpdatePhysicalCopy', () => {
  it('saves the draft through the outbox', async () => {
    mockEnqueueWrite.mockReturnValue('handle')
    mockSendWrite.mockResolvedValue('sent')
    const { result } = renderHook(() => useUpdatePhysicalCopy())
    await result.current('book-1', {
      pageCount: 320,
      textStartPage: 9,
      anchors: [{ page: 87, href: 'OEBPS/ch2.xhtml', offset: 9 }]
    })
    expect(mockEnqueueWrite).toHaveBeenCalledWith('write', {
      bookId: 'book-1',
      copy: {
        pageCount: 320,
        textStartPage: 9,
        anchors: [{ page: 87, position: { href: 'OEBPS/ch2.xhtml', offset: 9 } }]
      }
    })
    expect(mockSendWrite).toHaveBeenCalledWith('handle')
  })
})
