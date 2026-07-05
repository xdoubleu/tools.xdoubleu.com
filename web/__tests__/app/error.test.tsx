import { render, screen, fireEvent } from '@testing-library/react'
import ErrorBoundary from '@/app/error'

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
})
