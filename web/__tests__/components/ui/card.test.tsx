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

  it('pads with md by default', () => {
    const { container } = render(<Card>x</Card>)
    expect(container.firstChild).toHaveClass('p-4')
  })

  it('takes a padding size', () => {
    const { container, rerender } = render(<Card padding="none">x</Card>)
    expect(container.firstChild).not.toHaveClass('p-4')
    rerender(<Card padding="sm">x</Card>)
    expect(container.firstChild).toHaveClass('p-3')
    rerender(
      <Card variant="inset" padding="md">
        x
      </Card>
    )
    expect(container.firstChild).toHaveClass('p-4')
    expect(container.firstChild).not.toHaveClass('p-3')
  })
})
