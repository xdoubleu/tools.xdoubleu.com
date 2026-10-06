import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import OfflineBooksSettings, { formatStorageSize } from '@/components/books/OfflineBooksSettings'

const mockList = jest.fn()
const mockDeleteAll = jest.fn()
const listeners = new Set<() => void>()
jest.mock('@/lib/books/offlineBooks', () => ({
  listStoredBooks: () => mockList(),
  deleteAllStoredBooks: () => mockDeleteAll(),
  subscribeStoredBooks: (l: () => void) => {
    listeners.add(l)
    return () => listeners.delete(l)
  }
}))

const file = (bookId: string, format: string, size: number) => ({
  bookId,
  format,
  version: 'v',
  size,
  savedAt: 1
})

beforeEach(() => {
  jest.clearAllMocks()
  listeners.clear()
  mockDeleteAll.mockImplementation(async () => {
    mockList.mockResolvedValue([])
    listeners.forEach((l) => l())
  })
})

describe('formatStorageSize', () => {
  it.each([
    [0, '0 KB'],
    [512, '1 KB'],
    [1024 * 1024 - 1, '1024 KB'],
    [1024 * 1024, '1.0 MB'],
    [1024 * 1024 * 1024, '1.0 GB'],
    [1536 * 1024, '1.5 MB'],
    [3 * 1024 * 1024 * 1024, '3.0 GB']
  ])('formats %d bytes as %s', (bytes, text) => {
    expect(formatStorageSize(bytes)).toBe(text)
  })
})

describe('OfflineBooksSettings', () => {
  it('shows how many books are stored and their total size', async () => {
    mockList.mockResolvedValue([
      file('a', 'epub', 1024 * 1024),
      file('a', 'kepub', 1024 * 1024),
      file('b', 'pdf', 1024 * 1024)
    ])
    render(<OfflineBooksSettings />)
    expect(await screen.findByText('2 books stored on this device (3.0 MB).')).toBeInTheDocument()
    expect(screen.getByText(/added to the Home Screen/)).toBeInTheDocument()
  })

  it('uses the singular for one book', async () => {
    mockList.mockResolvedValue([file('a', 'epub', 2048)])
    render(<OfflineBooksSettings />)
    expect(await screen.findByText('1 book stored on this device (2 KB).')).toBeInTheDocument()
  })

  it('says when nothing is stored, with no remove action', async () => {
    mockList.mockResolvedValue([])
    render(<OfflineBooksSettings />)
    expect(await screen.findByText('No books stored on this device.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Remove offline books' })).not.toBeInTheDocument()
  })

  it('removes every stored book after confirming', async () => {
    mockList.mockResolvedValue([file('a', 'epub', 1024)])
    render(<OfflineBooksSettings />)
    fireEvent.click(await screen.findByRole('button', { name: 'Remove offline books' }))
    expect(mockDeleteAll).not.toHaveBeenCalled()
    expect(screen.getByRole('dialog')).toHaveTextContent(/downloaded again/)

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Remove' }))
    })

    expect(mockDeleteAll).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(await screen.findByText('No books stored on this device.')).toBeInTheDocument()
  })

  it('shows the removal as pending, and is ready for the next one', async () => {
    let finish!: () => void
    mockDeleteAll.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          finish = () => {
            listeners.forEach((l) => l())
            resolve()
          }
        })
    )
    mockList.mockResolvedValue([file('a', 'epub', 1024)])
    render(<OfflineBooksSettings />)
    fireEvent.click(await screen.findByRole('button', { name: 'Remove offline books' }))
    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))
    expect(await screen.findByRole('button', { name: 'Removing…' })).toBeDisabled()

    await act(async () => finish())
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

    // Books stored again later can be removed again.
    fireEvent.click(await screen.findByRole('button', { name: 'Remove offline books' }))
    expect(screen.getByRole('button', { name: 'Remove' })).toBeEnabled()
  })

  it('keeps the books when the confirmation is cancelled', async () => {
    mockList.mockResolvedValue([file('a', 'epub', 1024)])
    render(<OfflineBooksSettings />)
    fireEvent.click(await screen.findByRole('button', { name: 'Remove offline books' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(mockDeleteAll).not.toHaveBeenCalled()
  })

  it('stops listening once unmounted', async () => {
    mockList.mockResolvedValue([])
    const { unmount } = render(<OfflineBooksSettings />)
    await screen.findByText('No books stored on this device.')
    unmount()
    expect(listeners.size).toBe(0)
  })
})
