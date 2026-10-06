import { render } from '@testing-library/react'
import OfflineBooksSync from '@/components/books/OfflineBooksSync'

interface LibraryState {
  data?: { library?: object }
  error?: Error
  isValidating: boolean
}

let mockLibrary: LibraryState = { isValidating: false }
const mockUseLibrary = jest.fn<LibraryState, [unknown]>(() => mockLibrary)
jest.mock('@/hooks/useBooks', () => ({
  useLibrary: (config: unknown) => mockUseLibrary(config)
}))

let mockServedFromCache = false
const mockServedFromCacheFn = jest.fn<boolean, [unknown]>(() => mockServedFromCache)
jest.mock('@/lib/offline/persist', () => ({
  servedFromCache: (key: unknown) => mockServedFromCacheFn(key)
}))

const mockSync = jest.fn()
jest.mock('@/lib/books/offlineSync', () => ({
  syncOfflineBooks: (library: unknown) => mockSync(library)
}))

const library = { reading: [] }

beforeEach(() => {
  jest.clearAllMocks()
  mockServedFromCache = false
  mockSync.mockResolvedValue(undefined)
})

describe('OfflineBooksSync', () => {
  it('syncs once the library finishes loading live', () => {
    mockLibrary = { isValidating: true }
    const { rerender } = render(<OfflineBooksSync />)
    expect(mockSync).not.toHaveBeenCalled()

    mockLibrary = { data: { library }, isValidating: false }
    rerender(<OfflineBooksSync />)
    expect(mockSync).toHaveBeenCalledWith(library)
  })

  it('syncs again after each refresh', () => {
    mockLibrary = { data: { library }, isValidating: false }
    const { rerender } = render(<OfflineBooksSync />)
    mockLibrary = { data: { library }, isValidating: true }
    rerender(<OfflineBooksSync />)
    mockLibrary = { data: { library }, isValidating: false }
    rerender(<OfflineBooksSync />)
    expect(mockSync).toHaveBeenCalledTimes(2)
  })

  it('waits for a revalidation to finish before trusting the data', () => {
    mockLibrary = { data: { library }, isValidating: true }
    render(<OfflineBooksSync />)
    expect(mockSync).not.toHaveBeenCalled()
  })

  it('does nothing when the library failed to load', () => {
    mockLibrary = { data: { library }, error: new Error('boom'), isValidating: false }
    render(<OfflineBooksSync />)
    expect(mockSync).not.toHaveBeenCalled()
  })

  it('does nothing without library data', () => {
    mockLibrary = { data: {}, isValidating: false }
    render(<OfflineBooksSync />)
    expect(mockSync).not.toHaveBeenCalled()
  })

  it('does nothing with the saved library served in place of live data', () => {
    mockServedFromCache = true
    mockLibrary = { data: { library }, isValidating: false }
    render(<OfflineBooksSync />)
    expect(mockServedFromCacheFn).toHaveBeenCalledWith('/books')
    expect(mockSync).not.toHaveBeenCalled()
  })

  it('does not refetch the library on focus outside the books pages', () => {
    mockLibrary = { isValidating: true }
    render(<OfflineBooksSync />)
    expect(mockUseLibrary).toHaveBeenCalledWith({ revalidateOnFocus: false })
  })
})
