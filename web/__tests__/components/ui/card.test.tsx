import { render } from '@testing-library/react'
import { Card } from '@/components/ui/card'

describe('Card', () => {
  it('renders the raised surface by default', () => {
    const { container } = render(<Card>x</Card>)
    expect(container.firstChild).toHaveClass('bg-card', 'shadow-card')
  })

  it('renders a flat padded panel for variant="inset"', () => {
    const { container } = render(<Card variant="inset">x</Card>)
    expect(container.firstChild).toHaveClass('bg-surface', 'p-3')
    expect(container.firstChild).not.toHaveClass('shadow-card')
  })
})
