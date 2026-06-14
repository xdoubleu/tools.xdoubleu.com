import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ClearLibraryDialog from '@/components/backlog/ClearLibraryDialog'

const mockClearLibrary = jest.fn()
const mockMutate = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useClearLibrary: () => mockClearLibrary
}))

jest.mock('swr', () => ({
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

function renderDialog(open = true, onOpenChange = jest.fn(), onCleared = jest.fn()) {
  return render(
    <ClearLibraryDialog open={open} onOpenChange={onOpenChange} onCleared={onCleared} />
  )
}

describe('ClearLibraryDialog', () => {
  beforeEach(() => {
    mockClearLibrary.mockReset()
    mockMutate.mockReset()
  })

  it('renders the dialog when open', () => {
    renderDialog()
    expect(screen.getByText('Clear entire library')).toBeInTheDocument()
  })

  it('does not render content when closed', () => {
    renderDialog(false)
    expect(screen.queryByText('Clear entire library')).not.toBeInTheDocument()
  })

  it('confirm button is disabled when input is empty', () => {
    renderDialog()
    const btn = screen.getByTestId('clear-library-confirm-btn')
    expect(btn).toBeDisabled()
  })

  it('confirm button is disabled when input is wrong', () => {
    renderDialog()
    fireEvent.change(screen.getByTestId('clear-library-confirm-input'), {
      target: { value: 'delete' }
    })
    expect(screen.getByTestId('clear-library-confirm-btn')).toBeDisabled()
  })

  it('confirm button is enabled when DELETE is typed exactly', () => {
    renderDialog()
    fireEvent.change(screen.getByTestId('clear-library-confirm-input'), {
      target: { value: 'DELETE' }
    })
    expect(screen.getByTestId('clear-library-confirm-btn')).not.toBeDisabled()
  })

  it('calls clearLibrary and mutate on confirm', async () => {
    mockClearLibrary.mockResolvedValue({})
    mockMutate.mockResolvedValue(undefined)
    const onOpenChange = jest.fn()
    const onCleared = jest.fn()
    renderDialog(true, onOpenChange, onCleared)

    fireEvent.change(screen.getByTestId('clear-library-confirm-input'), {
      target: { value: 'DELETE' }
    })
    fireEvent.click(screen.getByTestId('clear-library-confirm-btn'))

    await waitFor(() => expect(mockClearLibrary).toHaveBeenCalledTimes(1))
    expect(mockMutate).toHaveBeenCalledWith('/backlog/books')
    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(onCleared).toHaveBeenCalled()
  })

  it('shows error message when clearLibrary fails', async () => {
    mockClearLibrary.mockRejectedValue(new Error('network error'))
    renderDialog()

    fireEvent.change(screen.getByTestId('clear-library-confirm-input'), {
      target: { value: 'DELETE' }
    })
    fireEvent.click(screen.getByTestId('clear-library-confirm-btn'))

    await waitFor(() => expect(screen.getByTestId('clear-library-error')).toBeInTheDocument())
    expect(screen.getByTestId('clear-library-error')).toHaveTextContent('Failed to clear library')
  })
})
