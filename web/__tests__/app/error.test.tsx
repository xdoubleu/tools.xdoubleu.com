import { render, screen, fireEvent, waitFor } from '@testing-library/react'

jest.mock('@sentry/nextjs', () => ({
  captureException: jest.fn()
}))

import * as Sentry from '@sentry/nextjs'
import ErrorBoundary from '@/app/error'

const mockCaptureException = jest.mocked(Sentry.captureException)

beforeEach(() => {
  jest.clearAllMocks()
})

describe('ErrorBoundary', () => {
  it('renders the digest when present and resets on click', () => {
    const reset = jest.fn()
    const error = Object.assign(new Error('boom'), { digest: 'abc123' })

    render(<ErrorBoundary error={error} reset={reset} />)

    expect(screen.getByText('Something went wrong')).toBeInTheDocument()
    expect(screen.getByText('Error reference: abc123')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(reset).toHaveBeenCalledTimes(1)
  })

  it('renders a generic message without a digest', () => {
    render(<ErrorBoundary error={new Error('boom')} reset={jest.fn()} />)
    expect(screen.getByText('An unexpected error occurred.')).toBeInTheDocument()
  })

  it('reports the error to Sentry on mount', async () => {
    const error = new Error('boom')
    render(<ErrorBoundary error={error} reset={jest.fn()} />)

    await waitFor(() => {
      expect(mockCaptureException).toHaveBeenCalledTimes(1)
      expect(mockCaptureException).toHaveBeenCalledWith(error)
    })
  })
})
