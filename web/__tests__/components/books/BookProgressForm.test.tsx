import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/books/v1/library_pb'

const mockUpdateProgress = jest.fn()
const mockMutate = jest.fn()

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

const mockPhysicalCopy: { data?: Record<string, unknown> } = { data: undefined }
const mockReadingState: { data: unknown } = { data: undefined }
const mockUpdatePhysicalCopy = jest.fn()
jest.mock('@/hooks/usePhysicalCopy', () => ({
  usePhysicalCopy: () => mockPhysicalCopy,
  useUpdatePhysicalCopy: () => mockUpdatePhysicalCopy
}))

jest.mock('@/hooks/useReadingState', () => ({
  useReadingState: () => mockReadingState
}))

jest.mock('@/hooks/useBooks', () => ({
  useUpdateProgress: () => mockUpdateProgress
}))

const mockTrack = jest.fn()
jest.mock('@/lib/analytics', () => ({
  track: (...args: unknown[]) => mockTrack(...args)
}))

import BookProgressForm from '@/components/books/BookProgressForm'

function makeBook(
  overrides: {
    progressMode?: string
    currentPage?: number
    progressPercent?: number
    tags?: string[]
    pageCount?: number
  } = {}
) {
  return create(UserBookSchema, {
    id: 'ub-1',
    bookId: 'book-1',
    status: 'currently-reading',
    progressMode: overrides.progressMode ?? 'pages',
    currentPage: overrides.currentPage ?? 50,
    progressPercent: overrides.progressPercent ?? 0,
    tags: overrides.tags ?? [],
    formats: [],
    book: create(BookSchema, {
      title: 'Test Book',
      authors: ['Author'],
      pageCount: overrides.pageCount ?? 200
    })
  })
}

describe('BookProgressForm', () => {
  beforeEach(() => {
    mockUpdateProgress.mockReset()
    mockMutate.mockReset()
    mockTrack.mockReset()
    mockUpdateProgress.mockResolvedValue({})
    mockUpdatePhysicalCopy.mockReset()
    mockUpdatePhysicalCopy.mockResolvedValue(undefined)
    mockPhysicalCopy.data = undefined
    mockReadingState.data = undefined
  })

  it('commits pages progress on Enter and calls UpdateProgress', async () => {
    render(<BookProgressForm userBook={makeBook({ progressMode: 'pages', currentPage: 50 })} />)

    const input = screen.getByLabelText('Current page')
    fireEvent.change(input, { target: { value: '120' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalledWith({
        bookId: 'book-1',
        progressMode: 'pages',
        currentPage: 120,
        progressPercent: 0,
        readAt: expect.any(String)
      })
    })
    expect(mockMutate).toHaveBeenCalledWith('/books')
  })

  it('commits percent progress when the Save button is clicked', async () => {
    render(
      <BookProgressForm
        userBook={makeBook({ progressMode: 'percent', progressPercent: 20, tags: ['own-digital'] })}
      />
    )

    const input = screen.getByLabelText('Progress percent')
    fireEvent.change(input, { target: { value: '75' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalledWith(
        expect.objectContaining({ progressPercent: 75 })
      )
    })
  })

  it('switches the input to percent mode when the Percent tab is chosen', () => {
    render(<BookProgressForm userBook={makeBook({ progressMode: 'pages' })} />)

    fireEvent.click(screen.getByRole('tab', { name: 'Percent' }))

    expect(screen.getByLabelText('Progress percent')).toBeInTheDocument()
    expect(screen.queryByLabelText('Current page')).not.toBeInTheDocument()
  })

  it('commits pages progress when the Save button is clicked', async () => {
    render(<BookProgressForm userBook={makeBook({ progressMode: 'pages', currentPage: 50 })} />)

    const input = screen.getByLabelText('Current page')
    fireEvent.change(input, { target: { value: '75' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalledWith(expect.objectContaining({ currentPage: 75 }))
    })
  })

  it('does not commit on blur', async () => {
    render(<BookProgressForm userBook={makeBook({ progressMode: 'pages', currentPage: 50 })} />)

    const input = screen.getByLabelText('Current page')
    fireEvent.change(input, { target: { value: '75' } })
    fireEvent.blur(input)

    expect(mockUpdateProgress).not.toHaveBeenCalled()
  })

  it('clicking Cancel calls onClose and resets without saving', () => {
    const onClose = jest.fn()
    render(<BookProgressForm userBook={makeBook({ currentPage: 50 })} onClose={onClose} />)

    fireEvent.change(screen.getByLabelText('Current page'), { target: { value: '99' } })
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onClose).toHaveBeenCalled()
    expect(mockUpdateProgress).not.toHaveBeenCalled()
  })

  it('disables Save and Cancel while a save is in flight', async () => {
    let resolveUpdate: () => void = () => {}
    mockUpdateProgress.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveUpdate = () => resolve({})
      })
    )
    render(<BookProgressForm userBook={makeBook()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(screen.getByRole('button', { name: 'Saving…' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled()
    resolveUpdate()
    await waitFor(() => expect(mockUpdateProgress).toHaveBeenCalledTimes(1))
  })

  it('calls onClose after a successful save', async () => {
    const onClose = jest.fn()
    render(<BookProgressForm userBook={makeBook()} onClose={onClose} />)

    fireEvent.keyDown(screen.getByLabelText('Current page'), { key: 'Enter' })

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled()
    })
  })

  it('calls onSaved after a successful save', async () => {
    const onSaved = jest.fn()
    render(<BookProgressForm userBook={makeBook()} onSaved={onSaved} />)

    fireEvent.keyDown(screen.getByLabelText('Current page'), { key: 'Enter' })

    await waitFor(() => {
      expect(onSaved).toHaveBeenCalled()
    })
  })

  it('pressing Escape calls onClose and resets without saving', () => {
    const onClose = jest.fn()
    render(<BookProgressForm userBook={makeBook({ currentPage: 50 })} onClose={onClose} />)

    const input = screen.getByLabelText('Current page')
    fireEvent.change(input, { target: { value: '99' } })
    fireEvent.keyDown(input, { key: 'Escape' })

    expect(onClose).toHaveBeenCalled()
    expect(mockUpdateProgress).not.toHaveBeenCalled()
  })

  it('pressing Escape without an onClose handler does not throw', () => {
    render(<BookProgressForm userBook={makeBook({ currentPage: 50 })} />)

    fireEvent.keyDown(screen.getByLabelText('Current page'), { key: 'Escape' })

    expect(mockUpdateProgress).not.toHaveBeenCalled()
  })

  it('ignores a second commit while a save is already in flight', async () => {
    let resolveUpdate: () => void = () => {}
    mockUpdateProgress.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveUpdate = () => resolve({})
      })
    )
    render(<BookProgressForm userBook={makeBook()} />)

    const input = screen.getByLabelText('Current page')
    fireEvent.keyDown(input, { key: 'Enter' })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(mockUpdateProgress).toHaveBeenCalledTimes(1)
    resolveUpdate()
    await waitFor(() => expect(mockUpdateProgress).toHaveBeenCalledTimes(1))
  })

  it('shows the page count next to the page field', () => {
    render(<BookProgressForm userBook={makeBook({ pageCount: 200 })} />)

    expect(screen.getByText('of 200 pages')).toBeInTheDocument()
  })

  it('omits the page count when the book has none', () => {
    render(<BookProgressForm userBook={makeBook({ pageCount: 0 })} />)

    expect(screen.queryByText(/^of /)).not.toBeInTheDocument()
    expect(screen.getByText('pages')).toBeInTheDocument()
  })

  it('shows a percent suffix in percent mode', () => {
    render(<BookProgressForm userBook={makeBook({ progressMode: 'percent' })} />)

    expect(screen.getByText('%')).toBeInTheDocument()
  })

  it('keeps the form open if the save fails', async () => {
    mockUpdateProgress.mockRejectedValueOnce(new Error('network error'))
    const onClose = jest.fn()
    render(<BookProgressForm userBook={makeBook()} onClose={onClose} />)

    fireEvent.keyDown(screen.getByLabelText('Current page'), { key: 'Enter' })

    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalled()
    })
    expect(onClose).not.toHaveBeenCalled()
  })

  it('selects the existing value on focus so retyping does not require clearing it first', () => {
    render(<BookProgressForm userBook={makeBook({ currentPage: 50 })} />)

    const input = screen.getByLabelText('Current page') as HTMLInputElement
    const selectSpy = jest.spyOn(input, 'select')
    fireEvent.focus(input)

    expect(selectSpy).toHaveBeenCalled()
  })

  it('tracks a saved update with its source', async () => {
    render(<BookProgressForm source="progress_cell" userBook={makeBook()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(mockTrack).toHaveBeenCalledWith('book_progress_updated', {
        source: 'progress_cell',
        progress_mode: 'pages'
      })
    )
  })

  it('tracks a cancel with its source', () => {
    render(<BookProgressForm source="progress_editor" userBook={makeBook()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(mockTrack).toHaveBeenCalledWith('book_progress_dialog_cancelled', {
      source: 'progress_editor'
    })
  })

  describe('physical copy', () => {
    const chapters = [
      { title: 'Chapter One', position: { href: 'OEBPS/ch1.xhtml', offset: 0 } },
      { title: 'Chapter Two', position: { href: 'OEBPS/ch2.xhtml', offset: 9 } }
    ]
    const physicalBook = () => makeBook({ tags: ['own-physical'], currentPage: 50 })

    beforeEach(() => {
      mockPhysicalCopy.data = {
        syncable: true,
        hasPrintPages: false,
        copy: { pageCount: 200, textStartPage: 1, anchors: [] },
        chapters
      }
    })

    it('shows where the digital reading left off and can use that page', () => {
      mockReadingState.data = {
        state: { source: 'kobo', physicalPage: 87, physicalExcerpt: 'It was a dark night.' }
      }
      render(<BookProgressForm userBook={physicalBook()} />)

      expect(screen.getByText('Continue on page ~87')).toBeInTheDocument()
      expect(screen.getByText('“It was a dark night.”')).toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'Use page 87' }))
      expect(screen.getByLabelText('Current page')).toHaveValue(87)
    })

    it('shows no hint when the page entered by hand is the newest', () => {
      mockReadingState.data = { state: { source: 'physical', physicalPage: 50 } }
      render(<BookProgressForm userBook={physicalBook()} />)
      expect(screen.queryByText(/Continue on page/)).not.toBeInTheDocument()
    })

    it('saves edited copy details before the page', async () => {
      render(<BookProgressForm userBook={physicalBook()} />)
      fireEvent.click(screen.getByRole('button', { name: /Copy details/ }))
      fireEvent.change(screen.getByLabelText('Pages in your copy'), { target: { value: '320' } })
      fireEvent.change(screen.getByLabelText('Text starts on page'), { target: { value: '9' } })
      fireEvent.click(screen.getByRole('button', { name: 'Add chapter anchor' }))
      fireEvent.change(screen.getByLabelText('Chapter'), {
        target: { value: 'OEBPS/ch2.xhtml#9' }
      })
      fireEvent.change(screen.getByLabelText('Page'), { target: { value: '87' } })
      fireEvent.click(screen.getByRole('button', { name: 'Add chapter anchor' }))
      expect(screen.getByText('of 320 pages')).toBeInTheDocument()

      fireEvent.click(screen.getByRole('button', { name: 'Save' }))
      await waitFor(() => expect(mockUpdateProgress).toHaveBeenCalled())
      expect(mockUpdatePhysicalCopy).toHaveBeenCalledWith('book-1', {
        pageCount: 320,
        textStartPage: 9,
        anchors: [{ page: 87, href: 'OEBPS/ch2.xhtml', offset: 9 }]
      })
      expect(mockUpdatePhysicalCopy.mock.invocationCallOrder[0]).toBeLessThan(
        mockUpdateProgress.mock.invocationCallOrder[0]
      )
      expect(mockMutate).toHaveBeenCalledWith(['/books/reading-state', 'book-1'])
    })

    it('removes an anchor', () => {
      mockPhysicalCopy.data = {
        syncable: true,
        hasPrintPages: false,
        copy: {
          pageCount: 200,
          textStartPage: 1,
          anchors: [{ page: 30, position: { href: 'OEBPS/ch2.xhtml', offset: 9 } }]
        },
        chapters
      }
      render(<BookProgressForm userBook={physicalBook()} />)
      fireEvent.click(screen.getByRole('button', { name: /Copy details/ }))
      expect(screen.getByLabelText('Page')).toHaveValue(30)
      fireEvent.click(screen.getByRole('button', { name: 'Remove chapter anchor' }))
      expect(screen.queryByLabelText('Page')).not.toBeInTheDocument()
    })

    it('explains that print page numbers map exactly', () => {
      mockPhysicalCopy.data = { ...mockPhysicalCopy.data, hasPrintPages: true }
      render(<BookProgressForm userBook={physicalBook()} />)
      fireEvent.click(screen.getByRole('button', { name: /Copy details/ }))
      expect(screen.getByText(/print edition's page numbers/)).toBeInTheDocument()
      expect(screen.queryByLabelText('Text starts on page')).not.toBeInTheDocument()
    })

    it('hides the copy details in percent mode and without an EPUB', () => {
      const { unmount } = render(
        <BookProgressForm
          userBook={makeBook({ tags: ['own-physical'], progressMode: 'percent' })}
        />
      )
      expect(screen.queryByRole('button', { name: /Copy details/ })).not.toBeInTheDocument()
      unmount()

      mockPhysicalCopy.data = { ...mockPhysicalCopy.data, syncable: false }
      render(<BookProgressForm userBook={physicalBook()} />)
      expect(screen.queryByRole('button', { name: /Copy details/ })).not.toBeInTheDocument()
    })
  })
})
