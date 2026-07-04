import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'

const mockEnableKoboSync = jest.fn()
const mockToggleTag = jest.fn()
const mockUseSWR = jest.fn()
const mockMutate = jest.fn()

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

jest.mock('@/hooks/useBooks', () => ({
  useEnableKoboSync: () => mockEnableKoboSync,
  useToggleTag: () => mockToggleTag,
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
    mockToggleTag.mockReset()
    mockUseSWR.mockReset()
    mockMutate.mockReset()
    setupSWR()
  })

  it('renders the Kobo sync checkbox', () => {
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} tags={[]} />)
    expect(screen.getByTestId('kobo-sync-checkbox')).toBeInTheDocument()
    expect(screen.getByLabelText('Kobo sync')).toBeInTheDocument()
  })

  it('disables the toggle when no epub or pdf is available', () => {
    setupSWR({ hasEpub: false, hasPdf: false })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} tags={[]} />)
    expect(screen.getByTestId('kobo-sync-checkbox')).toBeDisabled()
    expect(screen.getByText('Upload an EPUB or PDF to enable Kobo sync.')).toBeInTheDocument()
  })

  it('enables the toggle when epub is available', () => {
    setupSWR({ hasEpub: true })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} tags={[]} />)
    expect(screen.getByTestId('kobo-sync-checkbox')).not.toBeDisabled()
    expect(screen.queryByText('Upload an EPUB or PDF to enable Kobo sync.')).not.toBeInTheDocument()
  })

  it('enables the toggle when only pdf is available', () => {
    setupSWR({ hasEpub: false, hasPdf: true })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} tags={[]} />)
    expect(screen.getByTestId('kobo-sync-checkbox')).not.toBeDisabled()
    expect(screen.queryByText('Upload an EPUB or PDF to enable Kobo sync.')).not.toBeInTheDocument()
  })

  it('calls enableKoboSync when toggled on', async () => {
    setupSWR({ hasEpub: true })
    mockEnableKoboSync.mockResolvedValue({ kepubStatus: 'converting' })

    const onChanged = jest.fn()
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} tags={[]} onChanged={onChanged} />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-sync-checkbox'))
    })

    expect(mockEnableKoboSync).toHaveBeenCalledWith(BOOK_ID)
    expect(onChanged).toHaveBeenCalled()
  })

  it('calls toggleTag to disable kobo-sync when unchecked', async () => {
    setupSWR({ hasEpub: true, kepubStatus: 'ready' })
    mockToggleTag.mockResolvedValue({})

    const onChanged = jest.fn()
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={[]} onChanged={onChanged} />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-sync-checkbox'))
    })

    expect(mockToggleTag).toHaveBeenCalledWith(BOOK_ID, 'kobo-sync')
    expect(onChanged).toHaveBeenCalled()
  })

  it('shows "Preparing for Kobo..." when kepub_status is converting', () => {
    setupSWR({ hasEpub: true, kepubStatus: 'converting' })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={[]} />)
    expect(screen.getByTestId('kepub-status')).toHaveTextContent('Preparing for Kobo...')
  })

  it('shows "Ready to sync" when kepub_status is ready', () => {
    setupSWR({ hasEpub: true, kepubStatus: 'ready' })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={[]} />)
    expect(screen.getByTestId('kepub-status')).toHaveTextContent('Ready to sync')
  })

  it('shows "Conversion failed" when kepub_status is failed', () => {
    setupSWR({ hasEpub: true, kepubStatus: 'failed' })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={[]} />)
    expect(screen.getByTestId('kepub-status')).toHaveTextContent('Conversion failed')
  })

  it('does not show kepub status when not enabled', () => {
    setupSWR({ hasEpub: true, kepubStatus: 'ready' })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} tags={[]} />)
    expect(screen.queryByTestId('kepub-status')).not.toBeInTheDocument()
  })

  it('shows error message on enable failure', async () => {
    setupSWR({ hasEpub: true })
    mockEnableKoboSync.mockRejectedValue(new Error('Network error'))

    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} tags={[]} />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-sync-checkbox'))
    })

    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument()
    })
  })

  it('always passes bookId to useKEPUBStatus regardless of enabled state', () => {
    setupSWR({ hasEpub: true, kepubStatus: 'ready' })
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={[]} />)
    expect(mockUseSWR).toHaveBeenCalledWith(BOOK_ID)
  })

  it('passes bookId to useKEPUBStatus even when not enabled', () => {
    setupSWR()
    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} tags={[]} />)
    expect(mockUseSWR).toHaveBeenCalledWith(BOOK_ID)
  })

  it('checks immediately on click even though enabled prop never changes (responsiveness)', async () => {
    setupSWR({ hasEpub: true })
    mockEnableKoboSync.mockResolvedValue({ kepubStatus: 'converting' })

    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} tags={[]} />)
    const checkbox = screen.getByTestId('kobo-sync-checkbox')
    expect(checkbox).not.toBeChecked()

    await act(async () => {
      fireEvent.click(checkbox)
    })

    // prop stays false, but local state should have flipped
    expect(checkbox).toBeChecked()
  })

  it('unchecks immediately on click when disabling', async () => {
    setupSWR({ hasEpub: true, kepubStatus: 'ready' })
    mockToggleTag.mockResolvedValue({})

    render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={[]} />)
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

    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} tags={[]} />)
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

    render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} tags={[]} />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('kobo-sync-checkbox'))
    })

    expect(mockMutate).toHaveBeenCalledWith(['/books/kepub-status', BOOK_ID])
  })

  describe('PDF format selector', () => {
    it('shows format selector when sync is enabled and book has pdf', () => {
      setupSWR({ hasEpub: true, hasPdf: true, kepubStatus: 'ready' })
      render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={[]} />)
      expect(screen.getByTestId('kobo-format-kepub')).toBeInTheDocument()
      expect(screen.getByTestId('kobo-format-pdf')).toBeInTheDocument()
    })

    it('does not show format selector when book has no pdf', () => {
      setupSWR({ hasEpub: true, hasPdf: false, kepubStatus: 'ready' })
      render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={[]} />)
      expect(screen.queryByTestId('kobo-format-kepub')).not.toBeInTheDocument()
      expect(screen.queryByTestId('kobo-format-pdf')).not.toBeInTheDocument()
    })

    it('does not show format selector when sync is not enabled', () => {
      setupSWR({ hasEpub: true, hasPdf: true })
      render(<KoboSyncToggle bookId={BOOK_ID} enabled={false} tags={[]} />)
      expect(screen.queryByTestId('kobo-format-kepub')).not.toBeInTheDocument()
      expect(screen.queryByTestId('kobo-format-pdf')).not.toBeInTheDocument()
    })

    it('selects EPUB (converted) by default when no kobo-format-pdf tag', () => {
      setupSWR({ hasEpub: true, hasPdf: true, kepubStatus: 'ready' })
      render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={[]} />)
      expect(screen.getByTestId('kobo-format-kepub')).toBeChecked()
      expect(screen.getByTestId('kobo-format-pdf')).not.toBeChecked()
    })

    it('selects PDF (as-is) when kobo-format-pdf tag is present', () => {
      setupSWR({ hasEpub: true, hasPdf: true })
      render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={['kobo-format-pdf']} />)
      expect(screen.getByTestId('kobo-format-pdf')).toBeChecked()
      expect(screen.getByTestId('kobo-format-kepub')).not.toBeChecked()
    })

    it('toggles kobo-format-pdf tag when PDF radio is selected', async () => {
      setupSWR({ hasEpub: true, hasPdf: true, kepubStatus: 'ready' })
      mockToggleTag.mockResolvedValue({})
      const onChanged = jest.fn()

      render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={[]} onChanged={onChanged} />)

      await act(async () => {
        fireEvent.click(screen.getByTestId('kobo-format-pdf'))
      })

      expect(mockToggleTag).toHaveBeenCalledWith(BOOK_ID, 'kobo-format-pdf')
      expect(onChanged).toHaveBeenCalled()
    })

    it('toggles kobo-format-pdf tag and re-triggers enableKoboSync when EPUB radio is selected', async () => {
      setupSWR({ hasEpub: true, hasPdf: true, kepubStatus: 'ready' })
      mockToggleTag.mockResolvedValue({})
      mockEnableKoboSync.mockResolvedValue({ kepubStatus: 'converting' })
      const onChanged = jest.fn()

      render(
        <KoboSyncToggle
          bookId={BOOK_ID}
          enabled={true}
          tags={['kobo-format-pdf']}
          onChanged={onChanged}
        />
      )

      await act(async () => {
        fireEvent.click(screen.getByTestId('kobo-format-kepub'))
      })

      expect(mockToggleTag).toHaveBeenCalledWith(BOOK_ID, 'kobo-format-pdf')
      expect(mockEnableKoboSync).toHaveBeenCalledWith(BOOK_ID)
      expect(onChanged).toHaveBeenCalled()
    })

    it('hides kepub status when wantsPDF is true', () => {
      setupSWR({ hasEpub: true, hasPdf: true, kepubStatus: 'ready' })
      render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={['kobo-format-pdf']} />)
      expect(screen.queryByTestId('kepub-status')).not.toBeInTheDocument()
    })

    it('flips PDF radio to checked immediately (optimistic) on click', async () => {
      setupSWR({ hasEpub: true, hasPdf: true, kepubStatus: 'ready' })
      // Delay the mutation so we can observe the optimistic state.
      mockToggleTag.mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve({}), 200))
      )

      render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={[]} />)
      // Initially KEPUB is selected.
      expect(screen.getByTestId('kobo-format-kepub')).toBeChecked()
      expect(screen.getByTestId('kobo-format-pdf')).not.toBeChecked()

      // Click PDF — optimistic flip should happen before mutation resolves.
      fireEvent.click(screen.getByTestId('kobo-format-pdf'))

      expect(screen.getByTestId('kobo-format-pdf')).toBeChecked()
      expect(screen.getByTestId('kobo-format-kepub')).not.toBeChecked()
    })

    it('rolls back PDF radio on toggleTag failure', async () => {
      setupSWR({ hasEpub: true, hasPdf: true, kepubStatus: 'ready' })
      mockToggleTag.mockRejectedValue(new Error('network error'))

      render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={[]} />)
      expect(screen.getByTestId('kobo-format-kepub')).toBeChecked()

      await act(async () => {
        fireEvent.click(screen.getByTestId('kobo-format-pdf'))
      })

      await waitFor(() => {
        // Radio should roll back to KEPUB.
        expect(screen.getByTestId('kobo-format-kepub')).toBeChecked()
        expect(screen.getByTestId('kobo-format-pdf')).not.toBeChecked()
        expect(screen.getByText('network error')).toBeInTheDocument()
      })
    })

    it('flips KEPUB radio to checked immediately (optimistic) on click', async () => {
      setupSWR({ hasEpub: true, hasPdf: true, kepubStatus: 'ready' })
      mockToggleTag.mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve({}), 200))
      )
      mockEnableKoboSync.mockResolvedValue({ kepubStatus: 'converting' })

      render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={['kobo-format-pdf']} />)
      expect(screen.getByTestId('kobo-format-pdf')).toBeChecked()

      fireEvent.click(screen.getByTestId('kobo-format-kepub'))

      expect(screen.getByTestId('kobo-format-kepub')).toBeChecked()
      expect(screen.getByTestId('kobo-format-pdf')).not.toBeChecked()
    })

    it('rolls back KEPUB radio on enableKoboSync failure', async () => {
      setupSWR({ hasEpub: true, hasPdf: true, kepubStatus: 'ready' })
      mockToggleTag.mockResolvedValue({})
      mockEnableKoboSync.mockRejectedValue(new Error('sync error'))

      render(<KoboSyncToggle bookId={BOOK_ID} enabled={true} tags={['kobo-format-pdf']} />)
      expect(screen.getByTestId('kobo-format-pdf')).toBeChecked()

      await act(async () => {
        fireEvent.click(screen.getByTestId('kobo-format-kepub'))
      })

      await waitFor(() => {
        expect(screen.getByTestId('kobo-format-pdf')).toBeChecked()
        expect(screen.getByTestId('kobo-format-kepub')).not.toBeChecked()
        expect(screen.getByText('sync error')).toBeInTheDocument()
      })
    })
  })
})
