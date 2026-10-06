import { act, renderHook } from '@testing-library/react'

const mockRequest = jest.fn()
const mockUseKEPUBStatus = jest.fn()
const mockMutate = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  // A new function each render, like the real hook.
  useRequestKEPUBConversion: () => (bookId: string) => mockRequest(bookId),
  useKEPUBStatus: (...args: unknown[]) => mockUseKEPUBStatus(...args)
}))
jest.mock('swr', () => ({ mutate: (...args: unknown[]) => mockMutate(...args) }))

import { useKEPUBConversion } from '@/hooks/useKEPUBConversion'
import { swrKeys } from '@/lib/swrKeys'

// Like SWR, a null key reads nothing.
function setStatus(kepubStatus: string | undefined, error?: Error) {
  mockUseKEPUBStatus.mockImplementation((bookId: string | null) =>
    bookId
      ? { data: kepubStatus === undefined ? undefined : { kepubStatus }, error }
      : { data: undefined, error: undefined }
  )
}

/** Renders for `id` and lets the conversion request answer. */
async function renderAnswered(id: string | null = 'book-1') {
  const utils = renderHook(({ id }) => useKEPUBConversion(id), {
    initialProps: { id }
  })
  await act(async () => {})
  return utils
}

beforeEach(() => {
  jest.clearAllMocks()
  mockRequest.mockResolvedValue({ kepubStatus: 'converting' })
  setStatus(undefined)
})

describe('useKEPUBConversion', () => {
  it('does nothing without a book', async () => {
    const { result } = await renderAnswered(null)
    expect(mockRequest).not.toHaveBeenCalled()
    expect(mockUseKEPUBStatus).toHaveBeenLastCalledWith(null)
    expect(result.current).toBe('converting')
  })

  it('polls only after the request answers, seeded from its status', async () => {
    setStatus('ready')
    let resolve: (v: unknown) => void = () => {}
    mockRequest.mockReturnValueOnce(new Promise((r) => (resolve = r)))
    const { result, rerender } = renderHook(() => useKEPUBConversion('book-1'))
    rerender()
    // A stale KEPUB still reads as ready until the request replaces it.
    expect(result.current).toBe('converting')
    expect(mockUseKEPUBStatus).toHaveBeenLastCalledWith(null)
    await act(async () => resolve({ kepubStatus: 'converting' }))
    expect(mockRequest).toHaveBeenCalledTimes(1)
    expect(mockRequest).toHaveBeenCalledWith('book-1')
    expect(mockMutate).toHaveBeenCalledWith(
      swrKeys.kepubStatus('book-1'),
      expect.objectContaining({ kepubStatus: 'converting' }),
      { revalidate: false }
    )
    expect(mockUseKEPUBStatus).toHaveBeenLastCalledWith('book-1')
  })

  it('requests again after leaving and re-entering, waiting for the new answer', async () => {
    setStatus('ready')
    const { result, rerender } = await renderAnswered()
    expect(result.current).toBe('ready')
    rerender({ id: null })
    mockRequest.mockReturnValueOnce(new Promise(() => {}))
    rerender({ id: 'book-1' })
    expect(result.current).toBe('converting')
    expect(mockRequest).toHaveBeenCalledTimes(2)
  })

  it('reports converting until the status is ready', async () => {
    setStatus('')
    const { result, rerender } = await renderAnswered()
    expect(result.current).toBe('converting')
    setStatus('converting')
    rerender({ id: 'book-1' })
    expect(result.current).toBe('converting')
    setStatus('ready')
    rerender({ id: 'book-1' })
    expect(result.current).toBe('ready')
  })

  it('reports a failed conversion', async () => {
    setStatus('failed')
    expect((await renderAnswered()).result.current).toBe('failed')
  })

  it('reports a status read that fails before any status arrives', async () => {
    setStatus(undefined, new Error('internal'))
    expect((await renderAnswered()).result.current).toBe('failed')
  })

  it('keeps converting through a failed poll', async () => {
    setStatus('converting', new Error('offline'))
    expect((await renderAnswered()).result.current).toBe('converting')
  })

  it('reports a failed request', async () => {
    mockRequest.mockRejectedValue(new Error('internal'))
    const { result } = await renderAnswered()
    expect(result.current).toBe('failed')
    expect(mockMutate).not.toHaveBeenCalled()
    expect(mockUseKEPUBStatus).toHaveBeenLastCalledWith(null)
  })

  it('retries a failed request on re-entry', async () => {
    mockRequest.mockRejectedValueOnce(new Error('internal'))
    setStatus('converting')
    const { result, rerender } = await renderAnswered()
    expect(result.current).toBe('failed')
    rerender({ id: null })
    rerender({ id: 'book-1' })
    await act(async () => {})
    expect(result.current).toBe('converting')
    expect(mockRequest).toHaveBeenCalledTimes(2)
  })

  it('ignores answers that land after switching books', async () => {
    let resolve: (v: unknown) => void = () => {}
    let reject: (e: Error) => void = () => {}
    mockRequest
      .mockReturnValueOnce(new Promise((r) => (resolve = r)))
      .mockReturnValueOnce(new Promise((_, r) => (reject = r)))
      .mockReturnValueOnce(new Promise(() => {}))
    setStatus('ready')
    const { result, rerender } = renderHook(({ id }) => useKEPUBConversion(id), {
      initialProps: { id: 'book-1' }
    })
    rerender({ id: 'book-2' })
    rerender({ id: 'book-3' })
    await act(async () => {
      resolve({ kepubStatus: 'ready' })
      reject(new Error('late'))
    })
    expect(result.current).toBe('converting')
  })
})
