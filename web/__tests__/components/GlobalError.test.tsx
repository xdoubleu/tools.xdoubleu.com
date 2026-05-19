import { render, screen, fireEvent, waitFor } from '@testing-library/react'

jest.mock('@sentry/nextjs', () => ({
  captureException: jest.fn()
}))

import * as Sentry from '@sentry/nextjs'
import GlobalError from '@/app/global-error'

const mockCaptureException = Sentry.captureException as jest.Mock

beforeEach(() => {
  jest.clearAllMocks()
})

describe('GlobalError', () => {
  it('renders error UI with message', () => {
    const testError = new Error('Test error message')
    const mockReset = jest.fn()

    render(<GlobalError error={testError} reset={mockReset} />)

    expect(screen.getByText('Something went wrong')).toBeInTheDocument()
    expect(screen.getByText('Test error message')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Try again/ })).toBeInTheDocument()
  })

  it('calls captureException on mount with the error', () => {
    const testError = new Error('Specific test error')
    const mockReset = jest.fn()

    render(<GlobalError error={testError} reset={mockReset} />)

    waitFor(() => {
      expect(mockCaptureException).toHaveBeenCalledTimes(1)
      expect(mockCaptureException).toHaveBeenCalledWith(testError)
    })
  })

  it('re-calls captureException when error changes', async () => {
    const initialError = new Error('Initial error')
    const mockReset = jest.fn()

    const { rerender } = render(<GlobalError error={initialError} reset={mockReset} />)

    await waitFor(() => {
      expect(mockCaptureException).toHaveBeenCalledWith(initialError)
    })

    const updatedError = new Error('Updated error')
    rerender(<GlobalError error={updatedError} reset={mockReset} />)

    await waitFor(() => {
      expect(mockCaptureException).toHaveBeenCalledTimes(2)
      expect(mockCaptureException).toHaveBeenLastCalledWith(updatedError)
    })
  })

  it('calls reset when retry button is clicked', () => {
    const testError = new Error('Test error')
    const mockReset = jest.fn()

    render(<GlobalError error={testError} reset={mockReset} />)

    const retryButton = screen.getByRole('button', { name: /Try again/ })
    fireEvent.click(retryButton)

    expect(mockReset).toHaveBeenCalledTimes(1)
  })
})
