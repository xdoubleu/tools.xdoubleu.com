import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { ConnectError, Code } from '@connectrpc/connect'
import AddByUrlDialog from '@/components/reading/AddByUrlDialog'

const addByURL = jest.fn()
jest.mock('@/hooks/useBooks', () => ({
  useAddBookByURL: () => addByURL
}))
jest.mock('swr', () => ({ mutate: jest.fn() }))

describe('AddByUrlDialog', () => {
  beforeEach(() => addByURL.mockReset())

  it('submits the URL with the selected category and closes on success', async () => {
    addByURL.mockResolvedValue({ alreadyInLibrary: false })
    const onOpenChange = jest.fn()
    const onAdded = jest.fn()
    render(<AddByUrlDialog open onOpenChange={onOpenChange} onAdded={onAdded} />)

    fireEvent.change(screen.getByLabelText('URL'), {
      target: { value: 'https://arxiv.org/abs/2401.12345' }
    })
    fireEvent.change(screen.getByLabelText('Category'), {
      target: { value: 'paper' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() =>
      expect(addByURL).toHaveBeenCalledWith('https://arxiv.org/abs/2401.12345', 'paper')
    )
    await waitFor(() => expect(onAdded).toHaveBeenCalled())
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('reports an already-in-library paste without closing', async () => {
    addByURL.mockResolvedValue({ alreadyInLibrary: true })
    const onOpenChange = jest.fn()
    render(<AddByUrlDialog open onOpenChange={onOpenChange} />)

    fireEvent.change(screen.getByLabelText('URL'), {
      target: { value: 'https://blog.example.com/x' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    expect(await screen.findByText('Already in your library.')).toBeInTheDocument()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
  })

  it('shows a code-specific error message', async () => {
    addByURL.mockRejectedValue(new ConnectError('nope', Code.Unavailable))
    render(<AddByUrlDialog open onOpenChange={jest.fn()} />)

    fireEvent.change(screen.getByLabelText('URL'), {
      target: { value: 'https://gone.example.com/x' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    expect(
      await screen.findByText('The page could not be fetched — it may be down or paywalled.')
    ).toBeInTheDocument()
  })
})
