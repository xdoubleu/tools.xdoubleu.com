import { render, screen } from '@testing-library/react'
import { Alert } from '@/components/ui/alert'

describe('Alert', () => {
  it('announces danger immediately', () => {
    render(<Alert tone="danger">Broken</Alert>)
    expect(screen.getByRole('alert')).toHaveClass('text-danger')
  })

  it('announces other tones politely, defaulting to info', () => {
    render(<Alert>Heads up</Alert>)
    expect(screen.getByRole('status')).toHaveClass('bg-accent/10')
  })

  it('keeps an explicit role and merges className', () => {
    render(
      <Alert tone="success" role="note" className="mt-2">
        Saved
      </Alert>
    )
    const el = screen.getByRole('note')
    expect(el).toHaveClass('text-success', 'mt-2')
  })
})
