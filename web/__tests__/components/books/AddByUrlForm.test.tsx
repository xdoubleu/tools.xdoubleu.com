import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConnectError, Code } from '@connectrpc/connect'

const addByURL = jest.fn()
jest.mock('@/hooks/useBooks', () => ({
  useAddBookByURL: () => addByURL
}))
jest.mock('swr', () => ({ mutate: jest.fn() }))

import AddByUrlForm from '@/components/books/AddByUrlForm'

describe('AddByUrlForm', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('adds a URL and calls onDone when not already present', async () => {
    addByURL.mockResolvedValue({ alreadyInLibrary: false })
    const onAdded = jest.fn()
    const onDone = jest.fn()
    render(<AddByUrlForm onAdded={onAdded} onDone={onDone} />)

    fireEvent.change(screen.getByLabelText('URL'), {
      target: { value: 'https://blog.example.com/posts/why-tests-matter' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => {
      expect(addByURL).toHaveBeenCalledWith('https://blog.example.com/posts/why-tests-matter')
      expect(onAdded).toHaveBeenCalled()
      expect(onDone).toHaveBeenCalled()
    })
  })

  it('reports already-in-library without closing', async () => {
    addByURL.mockResolvedValue({ alreadyInLibrary: true })
    const onDone = jest.fn()
    render(<AddByUrlForm onDone={onDone} />)

    fireEvent.change(screen.getByLabelText('URL'), {
      target: { value: 'https://example.com/post' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => {
      expect(screen.getByText('Already in your library.')).toBeInTheDocument()
    })
    expect(onDone).not.toHaveBeenCalled()
  })

  it('shows a friendly message on a connect error', async () => {
    addByURL.mockRejectedValue(new ConnectError('nope', Code.InvalidArgument))
    render(<AddByUrlForm />)

    fireEvent.change(screen.getByLabelText('URL'), {
      target: { value: 'https://example.com/broken' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => {
      expect(screen.getByText(/no readable article content/i)).toBeInTheDocument()
    })
  })
})
