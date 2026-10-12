import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'

const mockEnableKoboSync = jest.fn()
const mockSetBookTag = jest.fn()
const mockUseSWR = jest.fn()
const mockMutate = jest.fn()

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

jest.mock('@/hooks/useBooks', () => ({
  useEnableKoboSync: () => mockEnableKoboSync,
  useSetBookTag: () => mockSetBookTag,
  useKEPUBStatus: (bookId: string | null) => mockUseSWR(bookId)
}))

import KoboSyncToggle from '@/components/books/KoboSyncToggle'

const BOOK_ID = 'book-uuid-1234'

function setupSWR(data: { hasEpub?: boolean; hasPdf?: boolean; kepubStatus?: string } = {}) {
  mockUseSWR.mockReturnValue({
    data: { hasEpub: false, hasPdf: false, kepubStatus: '', ...data }
  })
}

describe('KoboSyncToggle', () => {
  beforeEach(() => {
    mockEnableKoboSync.mockReset()
    mockSetBookTag.mockReset()
    mockUseSWR.mockReset()
    mockMutate.mockReset()
    setupSWR()
  })

  it('renders the Kobo sync checkbox', () => {
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} />)
    expect(screen.getByTestId('kobo-sync-checkbox')).toBeInTheDocument()
    expect(screen.getByLabelText('Kobo sync')).toBeInTheDocument()
  })

  it('disables the toggle when no epub or pdf is available', () => {
    setupSWR({ hasEpub: false, hasPdf: false })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} />)
    expect(screen.getByTestId('kobo-sync-checkbox')).toBeDisabled()
    expect(screen.getByText('Upload an EPUB or PDF to enable Kobo sync.')).toBeInTheDocument()
  })

  it('enables the toggle when epub is available', () => {
    setupSWR({ hasEpub: true })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} />)
    expect(screen.getByTestId('kobo-sync-checkbox')).not.toBeDisabled()
    expect(screen.queryByText('Upload an EPUB or PDF to enable Kobo sync.')).not.toBeInTheDocument()
  })

  it('enables the toggle when only pdf is available', () => {
    setupSWR({ hasEpub: false, hasPdf: true })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} />)
    expect(screen.getByTestId('kobo-sync-checkbox')).not.toBeDisabled()
    expect(screen.queryByText('Upload an EPUB or PDF to enable Kobo sync.')).not.toBeInTheDocument()
  })

  it('calls enableKoboSync when toggled on', async () => {
    setupSWR({ hasEpub: true })
    mockEnableKoboSync.mockResolvedValue({ kepubStatus: 'converting' })

    const onChanged = jest.fn()
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} onChanged={onChanged} />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-sync-checkbox'))
    })

    expect(mockEnableKoboSync).toHaveBeenCalledWith(BOOK_ID)
    expect(onChanged).toHaveBeenCalled()
  })

  it('calls setBookTag to disable kobo-sync when unchecked', async () => {
    setupSWR({ hasEpub: true, kepubStatus: 'ready' })
    mockSetBookTag.mockResolvedValue({})

    const onChanged = jest.fn()
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} onChanged={onChanged} />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-sync-checkbox'))
    })

    expect(mockSetBookTag).toHaveBeenCalledWith(BOOK_ID, 'kobo-sync', false)
    expect(onChanged).toHaveBeenCalled()
  })

  it('shows "Preparing for Kobo…" when kepub_status is converting', () => {
    setupSWR({ hasEpub: true, kepubStatus: 'converting' })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} />)
    expect(screen.getByTestId('kepub-status')).toHaveTextContent('Preparing for Kobo…')
  })

  it('shows "Ready to sync" when kepub_status is ready', () => {
    setupSWR({ hasEpub: true, kepubStatus: 'ready' })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} />)
    expect(screen.getByTestId('kepub-status')).toHaveTextContent('Ready to sync')
  })

  it('shows "Conversion failed" when kepub_status is failed', () => {
    setupSWR({ hasEpub: true, kepubStatus: 'failed' })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} />)
    expect(screen.getByTestId('kepub-status')).toHaveTextContent('Conversion failed')
  })

  it('does not show kepub status when not enabled', () => {
    setupSWR({ hasEpub: true, kepubStatus: 'ready' })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} />)
    expect(screen.queryByTestId('kepub-status')).not.toBeInTheDocument()
  })

  it('shows error message on enable failure', async () => {
    setupSWR({ hasEpub: true })
    mockEnableKoboSync.mockRejectedValue(new Error('Network error'))

    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-sync-checkbox'))
    })

    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument()
    })
  })

  it('always passes bookId to useKEPUBStatus regardless of enabled state', () => {
    setupSWR({ hasEpub: true, kepubStatus: 'ready' })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} />)
    expect(mockUseSWR).toHaveBeenCalledWith(BOOK_ID)
  })

  it('passes bookId to useKEPUBStatus even when not enabled', () => {
    setupSWR()
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} />)
    expect(mockUseSWR).toHaveBeenCalledWith(BOOK_ID)
  })

  it('checks immediately on click even though enabled prop never changes (responsiveness)', async () => {
    setupSWR({ hasEpub: true })
    mockEnableKoboSync.mockResolvedValue({ kepubStatus: 'converting' })

    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} />)
    const checkbox = screen.getByTestId('kobo-sync-checkbox')
    expect(checkbox).not.toBeChecked()

    await act(async () => {
      fireEvent.click(checkbox)
    })

    // The prop stays false, but local state flipped.
    expect(checkbox).toBeChecked()
  })

  it('unchecks immediately on click when disabling', async () => {
    setupSWR({ hasEpub: true, kepubStatus: 'ready' })
    mockSetBookTag.mockResolvedValue({})

    render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} />)
    const checkbox = screen.getByTestId('kobo-sync-checkbox')
    expect(checkbox).toBeChecked()

    await act(async () => {
      fireEvent.click(checkbox)
    })

    expect(checkbox).not.toBeChecked()
  })

  it('rolls back to unchecked when enableKoboSync fails', async () => {
    setupSWR({ hasEpub: true })
    mockEnableKoboSync.mockRejectedValue(new Error('Network error'))

    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} />)
    const checkbox = screen.getByTestId('kobo-sync-checkbox')

    await act(async () => {
      fireEvent.click(checkbox)
    })

    await waitFor(() => {
      expect(checkbox).not.toBeChecked()
      expect(screen.getByText('Network error')).toBeInTheDocument()
    })
  })

  it('revalidates kepub-status after successful enable', async () => {
    setupSWR({ hasEpub: true })
    mockEnableKoboSync.mockResolvedValue({ kepubStatus: 'converting' })

    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-sync-checkbox'))
    })

    expect(mockMutate).toHaveBeenCalledWith(['/books/kepub-status', BOOK_ID])
  })

  it('sends a PDF book as its KEPUB, with no format choice', () => {
    setupSWR({ hasPdf: true, kepubStatus: 'ready' })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} />)
    expect(screen.queryByRole('radiogroup')).not.toBeInTheDocument()
    expect(screen.getByTestId('kepub-status')).toHaveTextContent('Ready to sync')
  })
})
