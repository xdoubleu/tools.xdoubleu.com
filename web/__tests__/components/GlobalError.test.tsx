import { render, screen, fireEvent, waitFor } from '@testing-library/react'

jest.mock('@sentry/nextjs', () => ({
  captureException: jest.fn()
}))

import * as Sentry from '@sentry/nextjs'
import GlobalError from '@/app/global-error'

const mockCaptureException = jest.mocked(Sentry.captureException)

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

  it('calls captureException on mount with the error', async () => {
    const testError = new Error('Specific test error')
    const mockReset = jest.fn()

    render(<GlobalError error={testError} reset={mockReset} />)

    await waitFor(() => {
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

  it('does not call reset when retry button is clicked', () => {
    // reset() only clears client-side error-boundary state for a root-layout
    // error — it doesn't re-run the layout's own failed data fetch (issue
    // #852), so the retry button must trigger a real reload instead.
    // jsdom makes `window.location` non-configurable, so this only asserts
    // the regression it exists to prevent (falling back to reset()) rather
    // than asserting reload() fires.
    const testError = new Error('Test error')
    const mockReset = jest.fn()

    render(<GlobalError error={testError} reset={mockReset} />)

    const retryButton = screen.getByRole('button', { name: /Try again/ })
    expect(() => fireEvent.click(retryButton)).not.toThrow()

    expect(mockReset).not.toHaveBeenCalled()
  })
})
