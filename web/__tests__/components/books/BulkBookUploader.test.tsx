import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const mockUploadBookFile = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useUploadBookFile: () => mockUploadBookFile
}))

jest.mock('@/lib/books/zipFiles', () => ({
  // Keep in sync with MAX_UPLOAD_BYTES in web/lib/backlog/zipFiles.ts.
  MAX_UPLOAD_BYTES: 250 * 1024 * 1024,
  isBookFile: (f: File) =>
    f.name.toLowerCase().endsWith('.epub') || f.name.toLowerCase().endsWith('.pdf'),
  contentTypeForBook: (f: File) =>
    f.name.toLowerCase().endsWith('.epub') ? 'application/epub+zip' : 'application/pdf',
  filesFromDataTransfer: async (dt: DataTransfer) => Array.from(dt.files)
}))

// Keep in sync with MAX_UPLOAD_BYTES in web/lib/backlog/zipFiles.ts.
const MOCK_MAX_UPLOAD_BYTES = 250 * 1024 * 1024

import BulkBookUploader from '@/components/books/BulkBookUploader'

function makeFile(name: string, type = 'application/epub+zip'): File {
  const file = new File(['data'], name, { type })
  // jsdom does not implement arrayBuffer() — provide a stub so the component
  // can call file.arrayBuffer() without throwing.
  file.arrayBuffer = () => Promise.resolve(new Uint8Array([1, 2, 3]).buffer)
  return file
}

function makeOversizeFile(name: string, type = 'application/epub+zip'): File {
  const file = makeFile(name, type)
  // Override size to be over the limit without allocating real memory.
  Object.defineProperty(file, 'size', { value: MOCK_MAX_UPLOAD_BYTES + 1 })
  return file
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('BulkBookUploader', () => {
  beforeEach(() => {
    mockUploadBookFile.mockReset()
  })

  it('renders drop zone and browse buttons', () => {
    render(<BulkBookUploader />)
    expect(screen.getByTestId('drop-zone')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Browse files' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Browse folder' })).toBeInTheDocument()
  })

  it('renders file and folder inputs', () => {
    render(<BulkBookUploader />)
    expect(screen.getByTestId('file-input')).toBeInTheDocument()
    expect(screen.getByTestId('folder-input')).toBeInTheDocument()
  })

  it('ignores files with unsupported extensions', async () => {
    render(<BulkBookUploader />)
    const input = screen.getByTestId('file-input') as HTMLInputElement
    Object.defineProperty(input, 'files', {
      value: [makeFile('notes.txt', 'text/plain')],
      configurable: true
    })
    await act(async () => {
      fireEvent.change(input)
    })
    expect(mockUploadBookFile).not.toHaveBeenCalled()
  })

  it('uploads each book file individually and shows progress', async () => {
    mockUploadBookFile.mockResolvedValue({})

    render(<BulkBookUploader />)
    const input = screen.getByTestId('file-input') as HTMLInputElement
    Object.defineProperty(input, 'files', {
      value: [makeFile('a.epub'), makeFile('b.pdf', 'application/pdf')],
      configurable: true
    })

    await act(async () => {
      fireEvent.change(input)
    })

    await waitFor(() => {
      expect(screen.getByText(/2 \/ 2 uploaded/)).toBeInTheDocument()
      expect(screen.getByText('Done')).toBeInTheDocument()
    })

    expect(mockUploadBookFile).toHaveBeenCalledTimes(2)
    expect(mockUploadBookFile).toHaveBeenCalledWith(expect.objectContaining({ name: 'a.epub' }))
    expect(mockUploadBookFile).toHaveBeenCalledWith(expect.objectContaining({ name: 'b.pdf' }))
  })

  it('shows failed count and error message when one file fails', async () => {
    mockUploadBookFile
      .mockResolvedValueOnce({})
      .mockRejectedValueOnce(new Error('unsupported format'))

    render(<BulkBookUploader />)
    const input = screen.getByTestId('file-input') as HTMLInputElement
    Object.defineProperty(input, 'files', {
      value: [makeFile('good.epub'), makeFile('bad.epub')],
      configurable: true
    })

    await act(async () => {
      fireEvent.change(input)
    })

    await waitFor(() => {
      expect(screen.getByText(/1 failed/)).toBeInTheDocument()
      expect(screen.getByText('bad.epub: unsupported format')).toBeInTheDocument()
    })
    expect(screen.getByRole('button', { name: 'Import more' })).toBeInTheDocument()
  })

  it('shows Failed when all files fail', async () => {
    mockUploadBookFile.mockRejectedValue(new Error('network error'))

    render(<BulkBookUploader />)
    const input = screen.getByTestId('file-input') as HTMLInputElement
    Object.defineProperty(input, 'files', { value: [makeFile('book.epub')], configurable: true })

    await act(async () => {
      fireEvent.change(input)
    })

    await waitFor(() => {
      expect(screen.getByText('Failed')).toBeInTheDocument()
    })
    expect(screen.getByRole('button', { name: 'Import more' })).toBeInTheDocument()
  })

  it('resets to idle when Import more is clicked', async () => {
    mockUploadBookFile.mockRejectedValue(new Error('oops'))

    render(<BulkBookUploader />)
    const input = screen.getByTestId('file-input') as HTMLInputElement
    Object.defineProperty(input, 'files', { value: [makeFile('book.epub')], configurable: true })
    await act(async () => {
      fireEvent.change(input)
    })
    await waitFor(() => screen.getByRole('button', { name: 'Import more' }))

    fireEvent.click(screen.getByRole('button', { name: 'Import more' }))

    expect(screen.queryByText(/uploaded/)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Import more' })).not.toBeInTheDocument()
  })

  it('rejects oversize file immediately without uploading', async () => {
    render(<BulkBookUploader />)
    const input = screen.getByTestId('file-input') as HTMLInputElement
    Object.defineProperty(input, 'files', {
      value: [makeOversizeFile('huge.epub')],
      configurable: true
    })

    await act(async () => {
      fireEvent.change(input)
    })

    await waitFor(() => {
      expect(screen.getByText(/huge\.epub: file is too large \(max 250 MB\)/)).toBeInTheDocument()
    })
    expect(mockUploadBookFile).not.toHaveBeenCalled()
  })

  it('skips oversize file and still uploads valid ones', async () => {
    mockUploadBookFile.mockResolvedValue({})

    render(<BulkBookUploader />)
    const input = screen.getByTestId('file-input') as HTMLInputElement
    Object.defineProperty(input, 'files', {
      value: [makeOversizeFile('big.epub'), makeFile('small.epub')],
      configurable: true
    })

    await act(async () => {
      fireEvent.change(input)
    })

    await waitFor(() => {
      expect(screen.getByText(/1 \/ 2 uploaded/)).toBeInTheDocument()
      expect(screen.getByText(/1 failed/)).toBeInTheDocument()
      expect(screen.getByText(/big\.epub: file is too large/)).toBeInTheDocument()
    })
    expect(mockUploadBookFile).toHaveBeenCalledTimes(1)
    expect(mockUploadBookFile).toHaveBeenCalledWith(expect.objectContaining({ name: 'small.epub' }))
  })

  it('accepts drag-and-drop files and uploads each', async () => {
    mockUploadBookFile.mockResolvedValue({})

    render(<BulkBookUploader />)
    const dropZone = screen.getByTestId('drop-zone')

    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
    const dt = { files: [makeFile('dropped.epub')] } as unknown as DataTransfer
    await act(async () => {
      fireEvent.dragOver(dropZone)
      fireEvent.drop(dropZone, { dataTransfer: dt })
    })

    await waitFor(() => {
      expect(mockUploadBookFile).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'dropped.epub' })
      )
    })
  })
})
