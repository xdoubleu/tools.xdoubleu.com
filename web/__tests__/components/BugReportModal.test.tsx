import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import BugReportModal from '@/components/BugReportModal'

jest.mock('@/hooks/useBugReport', () => ({
  useCreateBugReport: jest.fn(() =>
    jest.fn().mockResolvedValue({ issueUrl: 'https://github.com/issue/123' })
  )
}))

import { useCreateBugReport } from '@/hooks/useBugReport'

const mockUseCreateBugReport = useCreateBugReport as jest.Mock

describe('BugReportModal', () => {
  const mockOnClose = jest.fn()

  beforeEach(() => {
    jest.clearAllMocks()
    mockOnClose.mockClear()
    mockUseCreateBugReport.mockReturnValue(
      jest.fn().mockResolvedValue({ issueUrl: 'https://github.com/issue/123' })
    )
  })

  it('does not render when isOpen is false', () => {
    const { container } = render(<BugReportModal isOpen={false} onClose={mockOnClose} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders form when isOpen is true', () => {
    render(<BugReportModal isOpen={true} onClose={mockOnClose} />)
    expect(screen.getByText('Report a Bug')).toBeInTheDocument()
    expect(screen.getByLabelText(/Title/)).toBeInTheDocument()
    expect(screen.getByLabelText(/Description/)).toBeInTheDocument()
  })

  it('has all form fields with correct attributes', () => {
    render(<BugReportModal isOpen={true} onClose={mockOnClose} />)

    const titleInput = screen.getByLabelText(/Title/) as HTMLInputElement
    expect(titleInput).toHaveAttribute('required')
    expect(titleInput).toHaveAttribute('type', 'text')

    const descriptionInput = screen.getByLabelText(/Description/) as HTMLTextAreaElement
    expect(descriptionInput).toHaveAttribute('required')

    const pageInput = screen.getByLabelText(/Page/) as HTMLInputElement
    expect(pageInput).toHaveAttribute('type', 'text')

    const consoleInput = screen.getByLabelText(/Console Logs/) as HTMLTextAreaElement
    expect(consoleInput).toBeInTheDocument()

    const wsInput = screen.getByLabelText(/WebSocket Log/) as HTMLTextAreaElement
    expect(wsInput).toBeInTheDocument()
  })

  it('initializes page field with pathname on open', async () => {
    render(<BugReportModal isOpen={true} onClose={mockOnClose} />)

    await waitFor(() => {
      const pageInput = screen.getByLabelText(/Page/) as HTMLInputElement
      // Page field should be populated with some value (pathname)
      expect(pageInput.value).toBeTruthy()
    })
  })

  it('prevents submission when title or description is empty', async () => {
    const mockCreateBugReport = jest
      .fn()
      .mockResolvedValue({ issueUrl: 'https://github.com/issue/000' })
    mockUseCreateBugReport.mockReturnValue(mockCreateBugReport)

    render(<BugReportModal isOpen={true} onClose={mockOnClose} />)
    const submitBtn = screen.getByRole('button', { name: /Submit Report/ })

    // Click submit without filling required fields - should not call createBugReport
    fireEvent.click(submitBtn)

    // Wait a bit to ensure no call was made
    await new Promise((resolve) => setTimeout(resolve, 50))

    expect(mockCreateBugReport).not.toHaveBeenCalled()
  })

  it('calls hook with form data on submit', async () => {
    const mockCreateBugReport = jest
      .fn()
      .mockResolvedValue({ issueUrl: 'https://github.com/issue/456' })
    mockUseCreateBugReport.mockReturnValue(mockCreateBugReport)

    render(<BugReportModal isOpen={true} onClose={mockOnClose} />)

    const titleInput = screen.getByLabelText(/Title/)
    const descriptionInput = screen.getByLabelText(/Description/)
    const pageInput = screen.getByLabelText(/Page/)
    const consoleInput = screen.getByLabelText(/Console Logs/)
    const wsInput = screen.getByLabelText(/WebSocket Log/)
    const submitBtn = screen.getByRole('button', { name: /Submit Report/ })

    fireEvent.change(titleInput, { target: { value: 'Bug Title' } })
    fireEvent.change(descriptionInput, { target: { value: 'Bug Description' } })
    fireEvent.change(pageInput, { target: { value: '/custom-page' } })
    fireEvent.change(consoleInput, { target: { value: 'error: null' } })
    fireEvent.change(wsInput, { target: { value: 'ws: connected' } })

    fireEvent.click(submitBtn)

    await waitFor(() => {
      expect(mockCreateBugReport).toHaveBeenCalledWith(
        'Bug Title',
        'Bug Description',
        '/custom-page',
        'error: null',
        'ws: connected'
      )
    })
  })

  it('shows success message after submission', async () => {
    const mockCreateBugReport = jest
      .fn()
      .mockResolvedValue({ issueUrl: 'https://github.com/issue/789' })
    mockUseCreateBugReport.mockReturnValue(mockCreateBugReport)

    render(<BugReportModal isOpen={true} onClose={mockOnClose} />)

    const titleInput = screen.getByLabelText(/Title/)
    const descriptionInput = screen.getByLabelText(/Description/)
    const submitBtn = screen.getByRole('button', { name: /Submit Report/ })

    fireEvent.change(titleInput, { target: { value: 'Test Bug' } })
    fireEvent.change(descriptionInput, { target: { value: 'Test Description' } })
    fireEvent.click(submitBtn)

    await waitFor(() => {
      expect(screen.getByText(/Thank you! Bug report submitted successfully/)).toBeInTheDocument()
    })
  })

  it('displays GitHub issue link in success message', async () => {
    const mockCreateBugReport = jest
      .fn()
      .mockResolvedValue({ issueUrl: 'https://github.com/issue/999' })
    mockUseCreateBugReport.mockReturnValue(mockCreateBugReport)

    render(<BugReportModal isOpen={true} onClose={mockOnClose} />)

    const titleInput = screen.getByLabelText(/Title/)
    const descriptionInput = screen.getByLabelText(/Description/)
    const submitBtn = screen.getByRole('button', { name: /Submit Report/ })

    fireEvent.change(titleInput, { target: { value: 'Test Bug' } })
    fireEvent.change(descriptionInput, { target: { value: 'Test Description' } })
    fireEvent.click(submitBtn)

    await waitFor(() => {
      const link = screen.getByRole('link', { name: /https:\/\/github.com\/issue\/999/ })
      expect(link).toBeInTheDocument()
      expect(link).toHaveAttribute('href', 'https://github.com/issue/999')
    })
  })

  it('shows error message on submission failure', async () => {
    const mockCreateBugReport = jest.fn().mockRejectedValue(new Error('Network error'))
    mockUseCreateBugReport.mockReturnValue(mockCreateBugReport)

    render(<BugReportModal isOpen={true} onClose={mockOnClose} />)

    const titleInput = screen.getByLabelText(/Title/)
    const descriptionInput = screen.getByLabelText(/Description/)
    const submitBtn = screen.getByRole('button', { name: /Submit Report/ })

    fireEvent.change(titleInput, { target: { value: 'Test Bug' } })
    fireEvent.change(descriptionInput, { target: { value: 'Test Description' } })
    fireEvent.click(submitBtn)

    await waitFor(() => {
      expect(screen.getByText(/Network error/)).toBeInTheDocument()
    })
  })

  it('closes modal and resets form after success', async () => {
    jest.useFakeTimers()

    const mockCreateBugReport = jest
      .fn()
      .mockResolvedValue({ issueUrl: 'https://github.com/issue/111' })
    mockUseCreateBugReport.mockReturnValue(mockCreateBugReport)

    render(<BugReportModal isOpen={true} onClose={mockOnClose} />)

    const titleInput = screen.getByLabelText(/Title/)
    const descriptionInput = screen.getByLabelText(/Description/)
    const submitBtn = screen.getByRole('button', { name: /Submit Report/ })

    fireEvent.change(titleInput, { target: { value: 'Test Bug' } })
    fireEvent.change(descriptionInput, { target: { value: 'Test Description' } })
    fireEvent.click(submitBtn)

    await waitFor(() => {
      expect(screen.getByText(/Thank you! Bug report submitted successfully/)).toBeInTheDocument()
    })

    act(() => {
      jest.advanceTimersByTime(2100)
    })

    await waitFor(() => {
      expect(mockOnClose).toHaveBeenCalled()
    })

    jest.useRealTimers()
  })

  it('closes modal when cancel button is clicked', () => {
    render(<BugReportModal isOpen={true} onClose={mockOnClose} />)

    const cancelBtn = screen.getByRole('button', { name: /Cancel/ })
    fireEvent.click(cancelBtn)

    expect(mockOnClose).toHaveBeenCalled()
  })

  it('closes modal when clicking outside', () => {
    const { container } = render(<BugReportModal isOpen={true} onClose={mockOnClose} />)

    const backdrop = container.querySelector('.fixed.inset-0')
    if (backdrop) {
      fireEvent.click(backdrop)
    }

    expect(mockOnClose).toHaveBeenCalled()
  })

  it('shows loading state while submitting', async () => {
    const mockCreateBugReport = jest.fn(
      () => new Promise((resolve) => setTimeout(() => resolve({ issueUrl: 'url' }), 100))
    )
    mockUseCreateBugReport.mockReturnValue(mockCreateBugReport)

    render(<BugReportModal isOpen={true} onClose={mockOnClose} />)

    const titleInput = screen.getByLabelText(/Title/)
    const descriptionInput = screen.getByLabelText(/Description/)
    const submitBtn = screen.getByRole('button', { name: /Submit Report/ }) as HTMLButtonElement

    fireEvent.change(titleInput, { target: { value: 'Test Bug' } })
    fireEvent.change(descriptionInput, { target: { value: 'Test Description' } })
    fireEvent.click(submitBtn)

    await waitFor(() => {
      expect(submitBtn).toHaveTextContent('Submitting...')
    })
  })

  it('has responsive modal width classes', () => {
    const { container } = render(<BugReportModal isOpen={true} onClose={mockOnClose} />)
    const modalContent = container.querySelector('.bg-card')
    expect(modalContent).toHaveClass('w-full', 'mx-4', 'max-w-md')
  })

  it('has responsive padding classes', () => {
    const { container } = render(<BugReportModal isOpen={true} onClose={mockOnClose} />)
    const modalContent = container.querySelector('.bg-card')
    expect(modalContent).toHaveClass('p-4', 'sm:p-6')
  })
})
